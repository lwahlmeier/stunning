#!/bin/bash
rm -rf build 
GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="-s -w -X main.version=${VERSION}" -a -o ./build/sclient-linux-amd64
GO111MODULE=on CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -mod=vendor -ldflags="-s -w -X main.version=${VERSION}" -a -o ./build/sclient-darwin-amd64
GO111MODULE=on CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=vendor -ldflags="-s -w -X main.version=${VERSION}" -a -o ./build/sclient-windows-amd64.exe
