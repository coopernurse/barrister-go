#!/bin/sh

set -e

export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin

git pull
./test.sh
