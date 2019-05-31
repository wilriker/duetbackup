#!/usr/bin/env fish

env GOOS=linux GOARCH=arm go build -o duetbackup-linux-arm duetbackup.go
env GOOS=linux GOARCH=arm GOARM=7 go build -o duetbackup-linux-armv7 duetbackup.go
env GOOS=linux go build -o duetbackup-linux_amd64 duetbackup.go
env GOOS=windows go build -o duetbackup-windows_amd64 duetbackup.go
env GOOS=darwin go build -o duetbackup-darwin_amd64 duetbackup.go
