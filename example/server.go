package main

import (
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
	"net/http"
)

type CalculatorImpl struct{}

func (c CalculatorImpl) Add(a float64, b float64) (float64, error) {
	return a + b, nil
}

func (c CalculatorImpl) Subtract(a float64, b float64) (float64, error) {
	return a - b, nil
}

func main() {
	idl := barrister.MustParseIdlJson([]byte(calc.IdlJsonRaw))
	svr := calc.NewJSONServer(idl, true, CalculatorImpl{})
	http.Handle("/", &svr)

	err := http.ListenAndServe(":9233", nil)
	if err != nil {
		panic(err)
	}
}
