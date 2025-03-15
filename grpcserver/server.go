package grpcserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	amqp "github.com/rabbitmq/amqp091-go"

	pb "file-scanner/proto"
)

type Server struct {
	pb.UnimplementedFileServiceServer
	S3Client *s3.Client
	Bucket   string
	RabbitConn *amqp.Connection
    RabbitChan *amqp.Channel
}

type ScanSession struct {
	SessionID string
	Files     map[string]bool
	Completed bool
}

// In-memory storage of scan sessions
var scanSessions = map[string]*ScanSession{}

func (s *Server) InitiateScan(ctx context.Context, req *pb.InitiateScanRequest) (*pb.InitiateScanResponse, error) {
	sessionId := uuid.New().String()
	files := make(map[string]bool)

	for _, fileId := range req.FileIds {
		files[fileId] = false
	}

	scanSessions[sessionId] = &ScanSession{
		SessionID: sessionId,
		Files:     files,
	}

	return &pb.InitiateScanResponse{
		SessionId: sessionId,
	}, nil
}

func (s *Server) UploadFile(stream pb.FileService_UploadFileServer) error {
	const partSizeLimit = 5 * 1024 * 1024 // 5MB
	var fileName string
	var sessionId string
	var buffer bytes.Buffer

	var parts []types.CompletedPart
	var partNumber int32 = 0
	bufferSize := int64(0)

	var uploadID *string
	var uploadInitialized bool

	// Iterate over the file chunks from the client
	for {
		req, err := stream.Recv()

		if err == io.EOF {
			// EOF reached, complete the multipart upload OR do a put based on file size.
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to receive chunk: %v", err)
		}

		fileName = req.FileName
		sessionId = req.SessionId

		// We buffer the chunks of file received from the stream
		buffer.Write(req.Content)
		bufferSize += int64(len(req.Content))

		// When buffer exceeds 5MB, s3 multipart upload
		if bufferSize >= partSizeLimit {

			if !uploadInitialized {
				fmt.Printf("Starting s3 multipart upload, buffer size: %d\n", bufferSize)
				createResp, err := s.S3Client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
					Bucket: &s.Bucket,
					Key:    &fileName,
				})
				if err != nil {
					return fmt.Errorf("Failed to create multipart upload: %v", err)
				}
				uploadID = createResp.UploadId
				uploadInitialized = true
			}

			fmt.Printf("Uploading part, buffer size: {}", bufferSize)
			partNumber++
			uploadResp, err := s.S3Client.UploadPart(context.TODO(), &s3.UploadPartInput{
				Bucket:     &s.Bucket,
				Key:        &fileName,
				PartNumber: aws.Int32(partNumber),
				UploadId:   uploadID,
				Body:       bytes.NewReader(buffer.Bytes()),
			})
			if err != nil {
				return fmt.Errorf("Failed to upload part %d: %v", partNumber, err)
			}

			parts = append(parts, types.CompletedPart{
				ETag:       uploadResp.ETag,
				PartNumber: aws.Int32(partNumber),
			})

			// Reset the buffer for the next part
			buffer.Reset()
			bufferSize = 0
		}
	}

	// Check if the file is small (less than 5MB) and upload it directly using PutObject
	if bufferSize > 0 && bufferSize < partSizeLimit && !uploadInitialized {

		_, err := s.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: &s.Bucket,
			Key:    &fileName,
			Body:   bytes.NewReader(buffer.Bytes()),
		})
		if err != nil {
			return fmt.Errorf("Failed to put small file: %v", err)
		}
		fmt.Println("Uploaded small file directly using PutObject")
	} else {
		// If the file is large, complete the multipart upload
		if uploadInitialized {
			if bufferSize > 0 {
				partNumber++
				uploadResp, err := s.S3Client.UploadPart(context.TODO(), &s3.UploadPartInput{
					Bucket:     &s.Bucket,
					Key:        &fileName,
					PartNumber: aws.Int32(partNumber),
					UploadId:   uploadID,
					Body:       bytes.NewReader(buffer.Bytes()),
				})
				if err != nil {
					return fmt.Errorf("failed to upload final part %d: %v", partNumber, err)
				}

				parts = append(parts, types.CompletedPart{
					ETag:       uploadResp.ETag,
					PartNumber: aws.Int32(partNumber),
				})
			}

			_, err := s.S3Client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
				Bucket:   &s.Bucket,
				Key:      &fileName,
				UploadId: uploadID,
				MultipartUpload: &types.CompletedMultipartUpload{
					Parts: parts,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to complete multipart upload: %v", err)
			}
			fmt.Println("Completed multipart upload")
		}
	}

	// File fully uploaded, now send message to RabbitMQ
	err := s.sendToRabbitMQ(sessionId, fileName)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.UploadStatus{
		Message: "File uploaded successfully!",
		Success: true,
	})
}

// sendToRabbitMQ sends a message to RabbitMQ with file info
func (s *Server) sendToRabbitMQ(sessionID, fileName string) error {
    message := fmt.Sprintf("Session: %s, File: %s", sessionID, fileName)
    err := s.RabbitChan.Publish(
        "",    // exchange
        "file_scan_queue", // routing key
        false, // mandatory
        false, // immediate
        amqp.Publishing{
            ContentType: "text/plain",
            Body:        []byte(message),
        },
    )
    if err != nil {
        log.Printf("Failed to publish message to RabbitMQ: %v", err)
        return err
    }

    log.Printf("Published message to RabbitMQ: %s", message)
    return nil
}
