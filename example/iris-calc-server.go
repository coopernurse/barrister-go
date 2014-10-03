package main

import (
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/barrister-go/example/calc"
	"github.com/coopernurse/barrister-go/iris"
	"github.com/karalabe/iris-go"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
)

const relayPort int = 55555
const app = "calc"

type CalculatorImpl struct{}

func (c CalculatorImpl) Add(a float64, b float64) (float64, error) {
	return a + b, nil
}

func (c CalculatorImpl) Subtract(a float64, b float64) (float64, error) {
	return a - b, nil
}

func start() {
	workers := runtime.GOMAXPROCS(0)

	idl := barrister.MustParseIdlJson([]byte(calc.IdlJsonRaw))
	svr := calc.NewJSONServer(idl, true, CalculatorImpl{})

	stop := make(chan bool)
	wg := new(sync.WaitGroup)

	for i := 0; i < workers; i++ {
		go runWorker(svr, wg, stop)
		wg.Add(1)
	}

	log.Printf("Started %d worker(s)\n", workers)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Stopping workers")
	for i := 0; i < workers; i++ {
		stop <- true
	}
	wg.Wait()
	log.Println("Exiting")
}

func runWorker(svr barrister.Server, wg *sync.WaitGroup, quit <-chan bool) {
	handler := &bariris.IrisHandler{svr}
	conn, err := iris.Register(relayPort, app, handler, nil)
	if err != nil {
		log.Fatalf("Register failed: %v.", err)
	}
	defer conn.Unregister()

	<-quit
	wg.Done()
	log.Println("Worker exiting")
}

func main() {
	log.Println("iris-calc-server start")
	start()
}
