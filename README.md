# file-scanner

Project to learn about grpc client side streaming. Purpose of project is to create a server implementation that will allow clients to send files over grpc and for the server to check if the files contain anything malicious e.g malware.

Generate the grpc files via protoc --go_out=. --go-grpc_out=. proto/file_upload.proto