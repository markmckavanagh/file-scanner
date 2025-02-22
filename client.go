package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "os"

	"google.golang.org/grpc"

    pb "file-scanner/proto"
)

func main() {
    
    conn, err := grpc.Dial("localhost:3000", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to server: %v", err)
    }
    defer conn.Close()

    client := pb.NewFileServiceClient(conn)

    file, err := os.Open("testfile.txt")
    if err != nil {
        log.Fatalf("Failed to open file: %v", err)
    }
    defer file.Close()

    stream, err := client.UploadFile(context.Background())
    if err != nil {
        log.Fatalf("Failed to create stream: %v", err)
    }

    buffer := make([]byte, 1024)
    for {
        n, err := file.Read(buffer)
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Fatalf("Failed to read file: %v", err)
        }

        err = stream.Send(&pb.FileChunk{
            FileName: "testfile.txt",
            Content:  buffer[:n],
        })
        if err != nil {
            log.Fatalf("Failed to send chunk: %v", err)
        }
    }

    res, err := stream.CloseAndRecv()
    if err != nil {
        log.Fatalf("Failed to receive response: %v", err)
    }

    fmt.Printf("Upload status: Success=%v, Message=%s\n", res.Success, res.Message)
}
