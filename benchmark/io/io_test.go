// +build benchmark

package io

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/aclisp/go-nano"
	"github.com/aclisp/go-nano/benchmark/testdata"
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/session"
)

const (
	addr = "127.0.0.1:13250" // local address
	conc = 10                // concurrent client count
)

//
type TestHandler struct {
	component.Base
	metrics int32
}

func (h *TestHandler) AfterInit() {
	ticker := time.NewTicker(time.Second)

	// metrics output ticker
	go func() {
		for range ticker.C {
			println("QPS", atomic.LoadInt32(&h.metrics))
			atomic.StoreInt32(&h.metrics, 0)
		}
	}()
}

func (h *TestHandler) Ping(s *session.Session, data *testdata.Ping) error {
	atomic.AddInt32(&h.metrics, 1)
	return s.Push("pong", &testdata.Pong{Content: data.Content})
}

func server() {
	components := &component.Components{}
	components.Register(&TestHandler{})
	nano.Listen(addr,
		nano.WithComponents(components),
		//nano.WithDebugMode(),
	)
}

func client() {
	c := NewConnector()

	chReady := make(chan struct{})
	c.OnConnected(func() {
		chReady <- struct{}{}
	})

	if err := c.Start(addr); err != nil {
		panic(err)
	}

	c.On("pong", func(data interface{}) {
		res := new(testdata.Pong)
		deserialize(data.([]byte), res)
	})

	<-chReady
	for {
		c.Notify("TestHandler.Ping", &testdata.Ping{})
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIO(t *testing.T) {
	go server()

	// wait server startup
	time.Sleep(time.Second)
	t.Log("server started")

	for i := 0; i < conc; i++ {
		go client()
	}

	sg := make(chan os.Signal, 1)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)

	<-sg

	t.Log("exit")
}
