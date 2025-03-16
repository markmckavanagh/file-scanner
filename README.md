# file-scanner

Purpose of project is to create a server that will allow clients to stream files over grpc, a scan of the files is then performed to check if they contain anything malicious e.g malware.

Technologies used:

- GRPC Client Side Streaming
- RabbitMQ
- Minio (Local S3)
- ClamAV (For anti virus scanning)
