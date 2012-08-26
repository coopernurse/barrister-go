#!/bin/sh

set -e

DIR="$( cd -P "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
go build -o $DIR/server $DIR/server.go

$DIR/server $1 &
trap "kill -9 $!" TERM
wait
