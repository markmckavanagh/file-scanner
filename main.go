package main

import (
	"log"
	"net"

	"file-scanner/fileupload"
	"file-scanner/grpcserver"

	"google.golang.org/grpc"

	pb "file-scanner/proto"
)

//TODO: Once the file has fully  been successfully uploaded to s3 we will need to enqueue a message to start the actual scanning of the file.
// //TODO: When running the server lets update to the make command to also spin up minio.

const (
    port = ":3001"
)

func main() {
    
    lis, err := net.Listen("tcp", port)
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()

    pb.RegisterFileServiceServer(grpcServer, &grpcserver.Server {
        S3Client: fileupload.NewS3Client(),
		Bucket:   "newbuck/", 
    })

    log.Printf("gRPC server listening on %v", port)
    
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
