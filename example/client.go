package main

import (
	"fmt"
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
)

func NewCalculatorProxy(url string) calc.Calculator {
	trans := &barrister.HttpTransport{Url: url}
	client := barrister.NewRemoteClient(trans, true)
	return calc.NewCalculatorProxy(client)
}

func main() {
	calc := NewCalculatorProxy("http://localhost:9233")

	res, err := calc.Add(51, 22.3)
	fmt.Printf("51+22.3=%f - err=%v\n", res, err)
}
