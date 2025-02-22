package main

import (
    "log"
    "net"
	"fmt"
	"io"
	"context"

    "google.golang.org/grpc"
	"github.com/google/uuid"

    pb "file-scanner/proto"
)

const (
    port = ":3000"
)


type Server struct {
    pb.UnimplementedFileServiceServer
}

type ScanSession struct {
    SessionID     string
    Files         map[string]bool
    Completed     bool
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
    var fileName string
    var fileData []byte

    for {
        chunk, err := stream.Recv()
        if err == io.EOF {

			log.Printf("File '%s' uploaded successfully! Total size: %d bytes", fileName, len(fileData))

            return stream.SendAndClose(&pb.UploadStatus{
                Success: true,
                Message: fmt.Sprintf("File '%s' uploaded successfully!", fileName),
            })
        }
        if err != nil {
            return err
        }

		log.Printf("Received chunk for file '%s' of size: %d bytes", chunk.GetFileName(), len(chunk.GetContent()))

        fileName = chunk.GetFileName()
        fileData = append(fileData, chunk.GetContent()...)
    }
}

func main() {
    
    lis, err := net.Listen("tcp", port)
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()

    pb.RegisterFileServiceServer(grpcServer, &Server{})

    log.Printf("gRPC server listening on %v", port)
    
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
