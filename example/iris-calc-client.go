package main

import (
	"flag"
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
	"github.com/coopernurse/barrister-go/iris"
	"github.com/karalabe/iris-go"
	"log"
	"math/rand"
	"time"
)

func main() {
	var num int
	flag.IntVar(&num, "n", 1000, "Number of requests to make")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	relayPort := 55555
	conn, err := iris.Connect(relayPort)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	trans := &bariris.IrisTransport{"calc", time.Second * 30, conn}
	client := barrister.NewRemoteClient(trans, true)
	proxy := calc.NewCalculatorProxy(client)

	ok := 0

	start := time.Now()
	for i := 0; i < num; i++ {
		x := rand.Float64()
		y := rand.Float64()
		res, err := proxy.Add(x, y)
		if err != nil {
			log.Fatal("Unexpected error: ", err)
		} else if res != (x + y) {
			log.Fatal("Unexpected result: ", res)
		} else {
			ok++
		}
	}
	elapsed := time.Now().UnixNano() - start.UnixNano()

	sec := float64(elapsed) / 1e9

	log.Printf("OK responses: %d\n", ok)
	log.Printf("Seconds: %.2f   Req/sec: %.2f\n", sec, float64(ok)/sec)
}
