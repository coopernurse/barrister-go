package main

import (
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
	"github.com/coopernurse/barrister-go/iris"
	"github.com/karalabe/iris-go"
	"log"
)

const relayPort int = 55555

type CalculatorImpl struct{}

func (c CalculatorImpl) Add(a float64, b float64) (float64, error) {
	return a + b, nil
}

func (c CalculatorImpl) Subtract(a float64, b float64) (float64, error) {
	return a - b, nil
}

func start() {
	app := "calc"

	idl := barrister.MustParseIdlJson([]byte(calc.IdlJsonRaw))
	svr := calc.NewJSONServer(idl, true, CalculatorImpl{})

	handler := &bariris.IrisHandler{svr}
	_, err := iris.Connect(relayPort, app, handler)
	if err != nil {
		log.Fatalf("connection failed: %v.", err)
	}

	c := make(chan bool)
	<-c
}

func main() {
	log.Println("iris-calc-server start")
	start()
}
