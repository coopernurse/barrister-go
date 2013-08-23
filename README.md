[![build status](https://secure.travis-ci.org/coopernurse/barrister-go.png)](http://travis-ci.org/coopernurse/barrister-go)

# Barrister for Go

This project contains Go bindings for the Barrister RPC system.

For information on how to write a Barrister IDL, please visit:

http://barrister.bitmechanic.com/

## Installation

```sh
# Install the barrister translator (IDL -> JSON)
# you need to be root (or use sudo)
pip install barrister

# Install barrister-go
go get github.com/coopernurse/barrister-go
go install github.com/coopernurse/barrister-go/idl2go
```

## Run example

```sh
# Generate Go code from calc.idl
cd $GOPATH/src/github.com/coopernurse/barrister-go/example
barrister calc.idl | $GOPATH/bin/idl2go -i -p calc

# Compile and run server in background
go run server.go &

# Compile and run client
go run client.go
```

## API documentation

http://godoc.org/github.com/coopernurse/barrister-go

## idl2go usage

idl2go generates a .go file based on the IDL JSON.  If the IDL contains namespaced 
enums or structs, the namespaced elements will be written to separate .go files.

Usage info: `idl2go -h`

Examples:

```sh
# Loads auth.json and generates ./auth/auth.go
idl2go -p auth auth.json

# Reads IDL JSON from STDIN and generates /tmp/designsvc/designsvc.go
idl2go -p designsvc -i -d /tmp
```
