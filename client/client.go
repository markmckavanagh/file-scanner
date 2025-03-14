package main

import (
    "context"
    "io"
    "log"
    "os"
    "path/filepath"
    "sync"
    "fmt"

	"google.golang.org/grpc"

    pb "file-scanner/proto"

    "crypto/sha256"
    "encoding/hex"
)

const chunkSize = 1024 * 1024 // 1MB per chunk
//const chunkSize = 6 * 1024 * 1024

type ProgressTracker struct {
    totalSize   int64
    uploaded    int64
    mu          sync.Mutex
}

func NewProgressTracker() *ProgressTracker {
    return &ProgressTracker{}
}

func (pt *ProgressTracker) AddFileSize(size int64) {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    pt.totalSize += size
}

func (pt *ProgressTracker) AddUploaded(bytes int64) {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    pt.uploaded += bytes
    progress := float64(pt.uploaded) / float64(pt.totalSize) * 100
    log.Printf("Progress: %.2f%% (%d/%d bytes uploaded)", progress, pt.uploaded, pt.totalSize)
}

func hashFileName(fileName string) string {
    hasher := sha256.New()
    hasher.Write([]byte(fileName))
    return hex.EncodeToString(hasher.Sum(nil))
}

func uploadFile(client pb.FileServiceClient, sessionId string, filePath string, wg *sync.WaitGroup, pt *ProgressTracker) {
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
    hashedFileName := hashFileName(fileName)
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
            FileName: hashedFileName,
            SessionId: sessionId,
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
    } else {
        log.Printf("File '%s' uploaded successfully (hashed to '%s'): %v", fileName, hashedFileName, res.Message)
    }
}

func walkAndCollectFiles(rootDir string, pt *ProgressTracker) []string {
    var files []string

    err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }

        if !d.IsDir() {
            fileInfo, err := os.Stat(path)
            if err != nil {
                return err
            }

            pt.AddFileSize(fileInfo.Size())
            files = append(files, path)
        }

        return nil
    })

    if err != nil {
        log.Fatalf("Error walking directory: %v", err)
    }

    return files
}

func initiateScan(client pb.FileServiceClient, hashedFileNames []string) (string, error) {
    req := &pb.InitiateScanRequest{
        FileIds: hashedFileNames,
    }

    res, err := client.InitiateScan(context.Background(), req)
    if err != nil {
        return "", fmt.Errorf("failed to initiate scan session: %v", err)
    }

    log.Printf("Scan session initiated. Session ID: %s", res.SessionId)
    return res.SessionId, nil
}


func main() {
    
    conn, err := grpc.Dial("localhost:3001", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to server: %v", err)
    }
    defer conn.Close()

    client := pb.NewFileServiceClient(conn)

    rootDir := "./test_files"

    pt := NewProgressTracker()
    files := walkAndCollectFiles(rootDir, pt)

    var hashedFileNames []string
    for _, file := range files {
        hashedFileNames = append(hashedFileNames, hashFileName(file))
    }

    sessionId, err := initiateScan(client, hashedFileNames)
    if err != nil {
        log.Fatalf("Failed to initiate scan session: %v", err)
    }

    var wg sync.WaitGroup
    for _, file := range files {
        wg.Add(1)
        go uploadFile(client, sessionId, file, &wg, pt)
    }
    wg.Wait()

    //log.Println("All files uploaded successfully!")
}
