package barrws

import (
	"github.com/coopernurse/barrister-go"
	"github.com/coopernurse/retina/ws"
	"log"
)

func NewRunner(servers map[string]*barrister.Server, url string, workers int) *Runner {
	return &Runner{
		Servers:  servers,
		Url:      url,
		Workers:  workers,
		Shutdown: make(chan bool),
	}
}

type Runner struct {
	Servers  map[string]*barrister.Server
	Url      string
	Workers  int
	Shutdown chan bool
}

func (me *Runner) Run() {
	handler := func(headers map[string][]string, body []byte) (map[string][]string, []byte) {
		queue, ok := headers["X-Hub-Queue"]
		if !ok || len(queue) < 1 {
			log.Println("barrister-retinaws: message missing X-Hub-Queue header")
			return map[string][]string{"X-Hub-Status": []string{"500"}}, []byte("Missing X-Hub-Queue header")
		} else {
			server, ok := me.Servers[queue[0]]
			if !ok {
				msg := "No server bound for queue: " + queue[0]
				log.Println("barrister-retinaws: " + msg)
				return map[string][]string{"X-Hub-Status": []string{"500"}}, []byte(msg)
			} else {
				bheaders := barrister.Headers{Request: headers, Response: make(map[string][]string)}
				bheaders.ReadCookies()
				response := server.InvokeBytes(bheaders, body)
				return bheaders.Response, response
			}
		}
	}
	retinaws.BackendServer(me.Url, me.Workers, handler, me.Shutdown)
}
