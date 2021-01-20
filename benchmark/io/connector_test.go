// +build !benchmark

package io

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/aclisp/go-nano"
	"github.com/aclisp/go-nano/benchmark/testdata"
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/session"
)

type Server struct {
	component.Base
}

func (h *Server) Ping(s *session.Session, data *testdata.Ping) error {
	return s.Push("pong", &testdata.Pong{Content: data.Content})
}

func (h *Server) PingPong(s *session.Session, data *testdata.Ping) error {
	return s.Response(&testdata.Pong{Content: data.Content})
}

func runServer(addr string) {
	components := &component.Components{}
	components.Register(&Server{})
	nano.Listen(addr, nano.WithComponents(components), nano.WithDebugMode())
}

func waitFor(addr string, timeout time.Duration) (err error) {
	time.Sleep(10 * time.Millisecond)
	begin := time.Now()
	for time.Since(begin) < timeout {
		var conn net.Conn
		if conn, err = net.Dial("tcp", addr); err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				time.Sleep(10 * time.Millisecond)
				continue
			}
		} else {
			conn.Close()
		}
		break
	}
	return
}

func runClient(addr string, t *testing.T) {
	c := NewConnector()
	ready := make(chan struct{})
	done := make(chan struct{})

	c.OnConnected(func() { close(ready) })
	cb := func(data interface{}) {
		res := new(testdata.Pong)
		if err := deserialize(data.([]byte), res); err != nil {
			t.Error(err)
		}
		t.Log(res.String())

		if res.Content == "request" {
			close(done)
		}
	}
	c.On("pong", cb)

	if err := c.Start(addr); err != nil {
		t.Error(err)
		return
	}
	<-ready

	if err := c.Notify("Server.Ping", &testdata.Ping{}); err != nil {
		t.Error(err)
	}
	if err := c.Notify("Server.Ping", &testdata.Ping{Content: "notify"}); err != nil {
		t.Error(err)
	}
	if err := c.Request("Server.PingPong", &testdata.Ping{}, cb); err != nil {
		t.Error(err)
	}
	if err := c.Request("Server.PingPong", &testdata.Ping{Content: "request"}, cb); err != nil {
		t.Error(err)
	}

	<-done
	c.Close()
}

func TestConnector(t *testing.T) {
	const addr = "127.0.0.1:13250"

	go runServer(addr)
	if err := waitFor(addr, time.Second); err != nil {
		t.Fatal(err)
	}

	runClient(addr, t)
	nano.Shutdown()
}
