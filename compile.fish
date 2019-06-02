#!/usr/bin/env fish

env GOOS=linux GOARCH=arm go build duetbackup.go
and tar czf duetbackup-linux_arm.tgz duetbackup LICENSE

env GOOS=linux GOARCH=arm64 go build duetbackup.go
and tar czf duetbackup-linux_arm64.tgz duetbackup LICENSE

env GOOS=linux go build duetbackup.go
and tar czf duetbackup-linux_amd64.tgz duetbackup LICENSE

env GOOS=windows go build -o duetbackup.exe duetbackup.go
and zip -r duetbackup-windows_amd64.zip duetbackup.exe LICENSE

env GOOS=darwin go build duetbackup.go
and tar czf duetbackup-darwin_amd64.tgz duetbackup LICENSE
