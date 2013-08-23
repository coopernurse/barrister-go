[![build status](https://secure.travis-ci.org/coopernurse/barrister-go.png)](http://travis-ci.org/coopernurse/barrister-go)

# Barrister for Go

This project contains Go bindings for the Barrister RPC system.

## Installation

    # Install the barrister translator (IDL -> JSON)
    # you need to be root (or use sudo)
    pip install barrister

    # Install barrister-go
    go get github.com/coopernurse/barrister-go
    go install github.com/coopernurse/barrister-go/idl2go

## Run example

    # Generate Go code from calc.idl
    cd $GOPATH/src/github.com/coopernurse/barrister-go/example
    barrister calc.idl | $GOPATH/bin/idl2go -i -calc

    # Compile and run server in background
    go run server.go &

    # Compile and run client
    go run client.go

