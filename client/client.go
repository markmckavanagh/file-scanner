package main

import (
    "context"
    "io"
    "log"
    "os"
    "path/filepath"
    "sync"

	"google.golang.org/grpc"

    pb "file-scanner/proto"
)

const chunkSize = 1024 * 1024 // 1MB per chunk

type ProgressTracker struct {
    totalSize   int64
    uploaded    int64
    mu          sync.Mutex
}

func NewProgressTracker(totalSize int64) *ProgressTracker {
    return &ProgressTracker{
        totalSize: totalSize,
    }
}

func (pt *ProgressTracker) AddUploaded(bytes int64) {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    pt.uploaded += bytes
    progress := float64(pt.uploaded) / float64(pt.totalSize) * 100
    log.Printf("Progress: %.2f%% (%d/%d bytes uploaded)", progress, pt.uploaded, pt.totalSize)
}

func uploadFiles(client pb.FileServiceClient, rootDir string, pt *ProgressTracker) {
    var wg sync.WaitGroup

    err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }

        if !d.IsDir() {
            wg.Add(1)
            go uploadFile(client, path, &wg, pt)
        }

        return nil
    })

    if err != nil {
        log.Fatalf("Error walking directory: %v", err)
    }

    wg.Wait()
}

func uploadFile(client pb.FileServiceClient, filePath string, wg *sync.WaitGroup, pt *ProgressTracker) {
    defer wg.Done()

    file, err := os.Open(filePath)
    if err != nil {
        log.Printf("Failed to open file %s: %v", filePath, err)
        return
    }
    defer file.Close()

    stream, err := client.UploadFile(context.Background())
    if err != nil {
        log.Fatalf("Failed to create stream: %v", err)
    }

    fileName := filepath.Base(filePath)
    buffer := make([]byte, chunkSize)

    for {
        n, err := file.Read(buffer)
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Printf("Error reading file %s: %v", filePath, err)
            return
        }

        err = stream.Send(&pb.FileChunk{
            FileName: fileName,
            Content:  buffer[:n],
        })
        if err != nil {
            log.Printf("Error sending chunk for file %s: %v", filePath, err)
            return
        }

         pt.AddUploaded(int64(n))

        log.Printf("Sent chunk of %d bytes for file %s", n, fileName)
    }

    res, err := stream.CloseAndRecv()
    if err != nil {
        log.Printf("Error closing stream for file %s: %v", fileName, err)
        return
    }

    log.Printf("File '%s' uploaded successfully: %v", fileName, res.Message)
}

func getTotalSize(rootDir string) (int64, error) {
    var totalSize int64

    err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }

        if !d.IsDir() {
            fileInfo, err := os.Stat(path)
            if err != nil {
                return err
            }
            totalSize += fileInfo.Size()
        }

        return nil
    })

    return totalSize, err
}



func main() {
    
    conn, err := grpc.Dial("localhost:3000", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to server: %v", err)
    }
    defer conn.Close()

    client := pb.NewFileServiceClient(conn)

    rootDir := "./test_files"

    totalSize, err := getTotalSize(rootDir)
    if err != nil {
        log.Fatalf("Failed to calculate total file size: %v", err)
    }
  
    log.Printf("Total file size: %d bytes", totalSize)

    pt := NewProgressTracker(totalSize)

    log.Printf("Walking through directory %s", rootDir)

    uploadFiles(client, rootDir, pt)

    log.Println("All files uploaded successfully!")
}
