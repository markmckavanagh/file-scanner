FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY client/ .
COPY proto/ ./proto
COPY test_files/ ./test_files/

RUN go build -o /usr/local/bin/grpc-client ./client.go

FROM alpine:latest

# Install glibc for running Go binaries built with glibc
RUN apk add --no-cache libc6-compat

COPY --from=builder /usr/local/bin/grpc-client /usr/local/bin/grpc-client
COPY --from=builder /app/test_files /app/test_files

# Install curl using apk (Alpine's package manager)
RUN apk update && apk add --no-cache curl bash

# Download wait-for-it script
RUN curl -o /usr/local/bin/wait-for-it https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh && \
    chmod +x /usr/local/bin/wait-for-it

# Clean up (remove apk cache to reduce image size)
RUN rm -rf /var/cache/apk/*

CMD ["wait-for-it", "grpc-server:50051", "--", "/usr/local/bin/grpc-client"]