FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy the server code and proto files
COPY . .

RUN go build -o /usr/local/bin/grpc-server ./main.go

FROM alpine:latest

# # Install glibc for running Go binaries built with glibc
RUN apk add --no-cache libc6-compat

COPY --from=builder /usr/local/bin/grpc-server /usr/local/bin/grpc-server

RUN apk update && apk add --no-cache curl bash

RUN curl -o /usr/local/bin/wait-for-it https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh && \
    chmod +x /usr/local/bin/wait-for-it

# Clean up (remove apk cache to reduce image size)
RUN rm -rf /var/cache/apk/*

CMD ["wait-for-it", "rabbitmq:5672", "--", "wait-for-it", "minio:9000", "--", "/usr/local/bin/grpc-server"]
