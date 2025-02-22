package main

import (
    "log"
    "net"
	"fmt"
	"io"

    "google.golang.org/grpc"

    pb "file-scanner/proto"
)

const (
    port = ":3000"
)


type Server struct {
    pb.UnimplementedFileServiceServer
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
