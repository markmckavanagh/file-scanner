package main

import (
	"context"
	"fmt"
	"log"
	"bytes"
	"strings"
	"io"
	"net"

	"file-scanner/fileupload"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	chunkSize int64 = 1024 * 1024 * 5 // 5MB
    clamAVHost string = "clamav"
    clamAVPort string = "3310"
)

type FileProcessor struct {
	s3Client *s3.Client
	bucket   string
}

func scanChunkWithClamAV(chunk []byte) error {

    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", clamAVHost, clamAVPort))
    if err != nil {
        return fmt.Errorf("failed to connect to ClamAV: %s", err)
    }
    defer conn.Close()

    // Send INSTREAM command to ClamAV
    _, err = conn.Write([]byte("zINSTREAM\000"))
    if err != nil {
        return fmt.Errorf("failed to write INSTREAM command: %s", err)
    }

    // Send the chunk size to ClamAV
    chunkLen := make([]byte, 4)
    chunkLen[0] = byte(len(chunk) >> 24) //moves the most significant byte to the first byte.
    chunkLen[1] = byte(len(chunk) >> 16) //moves the second most significant byte
    chunkLen[2] = byte(len(chunk) >> 8) //moves the third byte.
    chunkLen[3] = byte(len(chunk)) // gets the least significant byte

    _, err = conn.Write(chunkLen)
    if err != nil {
        return fmt.Errorf("failed to write chunk length: %s", err)
    }

    _, err = conn.Write(chunk)
    if err != nil {
        return fmt.Errorf("failed to write chunk: %s", err)
    }

    // End the stream by sending zero length chunk
    _, err = conn.Write([]byte{0, 0, 0, 0})
    if err != nil {
        return fmt.Errorf("failed to write end of stream: %s", err)
    }

    // Read ClamAV's response
    response := make([]byte, 4096)
    n, err := conn.Read(response)
    if err != nil {
        return fmt.Errorf("failed to read ClamAV response: %s", err)
    }

    if bytes.Contains(response[:n], []byte("FOUND")) {
        return fmt.Errorf("Virus detected: %s", string(response[:n]))
    }

    //TODO: I think regardless of whether a virus is found we should update a db with the file status and also delete the file from our system.
    //I think enqueueing another message here either smae queue different task or seperate queue for this.

    fmt.Println("Chunk scanned successfully")
    return nil
}

func (processor *FileProcessor) downloadAndProcessPart(partNum int, file string, offset int64, size int64) {

	rangeHeader := fmt.Sprintf("bytes=%d-%d", offset, offset+size-1)

	resp, err := processor.s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &processor.bucket,
		Key:    &file,
		Range:  &rangeHeader,
	})
	if err != nil {
		log.Printf("Failed to download part %d: %v", partNum, err)
	}
	defer resp.Body.Close()

	// Read the chunk data into memory
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		log.Printf("Failed to read part %d: %v", partNum, err)
	}

	chunkData := buf.Bytes()

    if err := scanChunkWithClamAV(chunkData); err != nil {
		log.Fatalf("ClamAV scan failed: %s", err)
	}
}

func (processor *FileProcessor) ScanFile(filePath string) error {
    log.Printf("Scanning file: %s", filePath)

	resp, err := processor.s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &processor.bucket,
		Key:    &filePath,
	})
	if err != nil {
		log.Fatalf("Failed to get object metadata: %v", err)
	}

	objectSize := *resp.ContentLength
	fmt.Printf("Object size: %d bytes\n", objectSize)

	numParts := (objectSize + chunkSize - 1) / chunkSize

	for i := 0; int64(i) < numParts; i++ {
		offset := int64(i) * chunkSize       
		partSize := chunkSize               
		if offset+partSize > objectSize { 
			partSize = objectSize - offset
		}

		processor.downloadAndProcessPart(i, filePath, offset, partSize)

		//TODO: We process each chunk sequentially for memory reasons however this could potentially be pretty damn slow for a massive file. Could
		//create go routines to process in parallel but means may be more memory hungry.
}
	fmt.Println("Download and processing completed successfully!")

	//TODO: After scanning we need to add a record to DB
	//TODO: We also want to delete the file from s3 I think both of these could be seperate queues?? As defo want to ensire file deleted.
    log.Printf("Finished scanning file: %s", filePath)
	return nil
}

func (processor *FileProcessor) ProcessMessage(msg amqp.Delivery) {
    log.Printf("Received message: %s", msg.Body)

    // Assumes the message is of the format: "Session: sessionID, File: /path/to/file"
    message := string(msg.Body)
    parts := strings.Split(message, ",")
    if len(parts) < 2 {
        log.Printf("Invalid message format")
        return
    }

    // Extract file path from message
    filePart := strings.TrimSpace(parts[1])
    filePath := strings.TrimPrefix(filePart, "File: ")

    if err := processor.ScanFile(filePath); err != nil {
        log.Printf("Error scanning file: %v", err)
    } else {
        log.Printf("File %s scanned successfully", filePath)
    }

    // Ack message processed
    msg.Ack(false)
}

func main() {
    
    conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
    if err != nil {
        log.Fatalf("Failed to connect to RabbitMQ: %v", err)
    }
    defer conn.Close()

    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Failed to open a channel: %v", err)
    }
    defer ch.Close()

    q, err := ch.QueueDeclare(
        "file_scan_queue", // queue name
        true, // durable
        false, // delete when unused
        false, // exclusive
        false, // no-wait
        nil, // arguments
    )
    if err != nil {
        log.Fatalf("Failed to declare a queue: %v", err)
    }

    msgs, err := ch.Consume(
        q.Name, // queue
        "",     // consumer
        false,  // auto-ack (manual ack to ensure processing)
        false,  // exclusive
        false,  // no-local
        false,  // no-wait
        nil,    // args
    )
    if err != nil {
        log.Fatalf("Failed to register a consumer: %v", err)
    }

    log.Println("Waiting for messages...")

    processor := FileProcessor{
        s3Client: fileupload.NewS3Client(),
		bucket: "newbuck/",
    }

    // Run a goroutine to process incoming messages
    go func() {
        for msg := range msgs {
            processor.ProcessMessage(msg)
        }
    }()

    // Block the main thread to keep the application running
    select {}
}
