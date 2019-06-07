#!/usr/bin/env fish

set gofile cmd/duetbackup/main.go
set out duetbackup

env GOOS=linux GOARCH=arm go build -o $out $gofile
and tar czf duetbackup-linux_arm.tgz duetbackup LICENSE

env GOOS=linux GOARCH=arm64 go build -o $out $gofile
and tar czf duetbackup-linux_arm64.tgz duetbackup LICENSE

env GOOS=windows go build -o $out.exe $gofile
and zip -r duetbackup-windows_amd64.zip duetbackup.exe LICENSE

env GOOS=darwin go build -o $out $gofile
and tar czf duetbackup-darwin_amd64.tgz duetbackup LICENSE

env GOOS=linux go build -o $out $gofile
and tar czf duetbackup-linux_amd64.tgz duetbackup LICENSE
