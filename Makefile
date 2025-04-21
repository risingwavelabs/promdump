
###################################################
### Promdump
###################################################

upload-promdump:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o upload/promdump/Darwin/x86_64/promdump cmd/promdump/main.go
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o upload/promdump/Darwin/arm64/promdump cmd/promdump/main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o upload/promdump/Linux/x86_64/promdump cmd/promdump/main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=386   go build -o upload/promdump/Linux/i386/promdump cmd/promdump/main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o upload/promdump/Linux/arm64/promdump cmd/promdump/main.go
	chmod +x upload/promdump/Darwin/x86_64/promdump
	chmod +x upload/promdump/Darwin/arm64/promdump
	chmod +x upload/promdump/Linux/x86_64/promdump
	chmod +x upload/promdump/Linux/i386/promdump
	chmod +x upload/promdump/Linux/arm64/promdump
	cp scripts/download-promdump.sh upload/promdump/download.sh
	aws s3 cp --recursive upload/promdump/ s3://wavekit-release/promdump/

###################################################
### Prompush
###################################################

upload-prompush:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o upload/prompush/Darwin/x86_64/prompush cmd/prompush/main.go
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o upload/prompush/Darwin/arm64/prompush cmd/prompush/main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o upload/prompush/Linux/x86_64/prompush cmd/prompush/main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=386   go build -o upload/prompush/Linux/i386/prompush cmd/prompush/main.go
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o upload/prompush/Linux/arm64/prompush cmd/prompush/main.go
	chmod +x upload/prompush/Darwin/x86_64/prompush
	chmod +x upload/prompush/Darwin/arm64/prompush
	chmod +x upload/prompush/Linux/x86_64/prompush
	chmod +x upload/prompush/Linux/i386/prompush
	chmod +x upload/prompush/Linux/arm64/prompush
	cp scripts/download-prompush.sh upload/prompush/download.sh
	aws s3 cp --recursive upload/prompush/ s3://wavekit-release/prompush/
