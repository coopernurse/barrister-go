package main

import (
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
	"github.com/coopernurse/barrister-go/iris"
	"github.com/karalabe/iris-go"
	"log"
	"time"
)

func main() {
	relayPort := 55555
	conn, err := iris.Connect(relayPort, "", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	trans := &bariris.IrisTransport{"calc", time.Second * 30, conn}
	client := barrister.NewRemoteClient(trans, true)
	proxy := calc.NewCalculatorProxy(client)

	for i := 0; i < 10000; i++ {
		res, err := proxy.Add(1, 2)
		if err != nil {
			log.Fatal("Unexpected error: ", err)
		} else if res != 3 {
			log.Fatal("Unexpected result: ", res)
		}
	}
}
