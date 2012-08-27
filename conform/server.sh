#!/bin/sh

set -e

go build -o $BARRISTER_GO/conform/server $BARRISTER_GO/conform/server.go

$BARRISTER_GO/conform/server $1 &
trap "kill -9 $!" TERM
wait
