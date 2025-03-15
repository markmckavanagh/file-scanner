package main

import (
    "strings"
	"log"

    amqp "github.com/rabbitmq/amqp091-go"
)

func FileScanner(filePath string) error {
    log.Printf("Scanning file: %s", filePath)

	//TODO: After scanning we need to add a record to DB
	//TODO: We also want to delete the file from s3 I think both of these could be seperate queues?? As defo want to ensire file deleted.
    log.Printf("Finished scanning file: %s", filePath)
    return nil
}

func ProcessMessage(msg amqp.Delivery) {
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

    if err := FileScanner(filePath); err != nil {
        log.Printf("Error scanning file: %v", err)
    } else {
        log.Printf("File %s scanned successfully", filePath)
    }

    // Ack message processed
    msg.Ack(false)
}

func main() {
    
    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
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

    // Run a goroutine to process incoming messages
    go func() {
        for msg := range msgs {
            ProcessMessage(msg)
        }
    }()

    // Block the main thread to keep the application running
    select {}
}
