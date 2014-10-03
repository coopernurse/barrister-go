package bariris

import (
	"github.com/coopernurse/barrister-go"
	"github.com/karalabe/iris-go"
	"time"
)

type IrisTransport struct {
	App     string
	Timeout time.Duration
	Conn    *iris.Connection
}

func (t *IrisTransport) Send(in []byte) ([]byte, error) {
	return t.Conn.Request(t.App, in, t.Timeout)
}

type IrisHandler struct {
	Server barrister.Server
}

func (h *IrisHandler) Init(conn *iris.Connection) error { return nil }

func (h *IrisHandler) HandleBroadcast(msg []byte) {
	panic("Broadcast passed to request handler")
}

func (h *IrisHandler) HandleRequest(req []byte) ([]byte, error) {
	headers := barrister.Headers{}
	return h.Server.InvokeBytes(headers, req), nil
}

func (h *IrisHandler) HandleTunnel(tun *iris.Tunnel) {
	panic("Inbound tunnel on request handler")
}

func (h *IrisHandler) HandleDrop(reason error) {
	panic("Connection dropped on request handler")
}
