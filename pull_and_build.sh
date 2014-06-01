#!/bin/sh

set -e

export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin

mkdir -p $GOPATH
go get -u github.com/couchbaselabs/go.assert

git pull
./test.sh
