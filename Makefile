# Paths
PROTO_DIR=proto
PROTO_FILE=$(PROTO_DIR)/file_upload.proto
GENERATED_GO_FILES=$(PROTO_DIR)/*.pb.go

# gRPC file generation command
PROTOC_GEN_GRPC_GO=protoc --go_out=. --go-grpc_out=. $(PROTO_FILE)

# Spin up server
start: generate
	@echo "Starting gRPC server..."
	go run main.go

# Spin up client
client: generate
	@echo "Starting gRPC client..."
	go run client.go

# Sort GRPC shiz
generate:
	@echo "Regenerating gRPC files..."
	rm -f $(GENERATED_GO_FILES) # Delete existing generated files
	@echo "Generating new gRPC files..."
	$(PROTOC_GEN_GRPC_GO) $(PROTO_FILE)

# Wipe generated grpc files
clean:
	@echo "Cleaning up generated files..."
	rm -f $(GENERATED_GO_FILES)

# Build an executable
build: generate
	@echo "Building the project..."
	go build -o server .
