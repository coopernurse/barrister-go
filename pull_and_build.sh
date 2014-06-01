#!/bin/sh

set -e

export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin

mkdir -p $GOPATH/github.com/coopernurse
rm -f $GOPATH/github.com/coopernurse/barrister-go
ln -s `pwd` $GOPATH/github.com/coopernurse/barrister-go

go get -u github.com/couchbaselabs/go.assert

git pull
./test.sh
