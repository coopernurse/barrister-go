#!/bin/sh

set -e 

go clean
go test -v
go build conform/generated/conform.go
go build conform/client.go
go build conform/server.go