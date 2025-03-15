package main

import (
	"log"
	"net"

	"file-scanner/fileupload"
	"file-scanner/grpcserver"

	"google.golang.org/grpc"
	amqp "github.com/rabbitmq/amqp091-go"

	pb "file-scanner/proto"
)

//TODO: Once the file has fully  been successfully uploaded to s3 we will need to enqueue a message to start the actual scanning of the file.
// //TODO: When running the server lets update to the make command to also spin up minio.

const (
    port = ":3001"
)

func main() {

	// Connect to RabbitMQ
    rabbitConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
    if err != nil {
        log.Fatalf("Failed to connect to RabbitMQ: %v", err)
    }
    defer rabbitConn.Close()

	// Open a channel
    rabbitChan, err := rabbitConn.Channel()
    if err != nil {
        log.Fatalf("Failed to open a channel: %v", err)
    }
    defer rabbitChan.Close()

    // Declare a queue for file uploads
    _, err = rabbitChan.QueueDeclare(
        "file_scan_queue", // name
        true,                 // durable
        false,                // delete when unused
        false,                // exclusive
        false,                // no-wait
        nil,                  // arguments
    )
    if err != nil {
        log.Fatalf("Failed to declare a queue: %v", err)
    }
    
    lis, err := net.Listen("tcp", port)
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()

    pb.RegisterFileServiceServer(grpcServer, &grpcserver.Server {
        S3Client: fileupload.NewS3Client(),
		Bucket:   "newbuck/", 
		RabbitConn: rabbitConn,
		RabbitChan: rabbitChan,
    })

    log.Printf("gRPC server listening on %v", port)
    
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
