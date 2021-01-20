// +build benchmark

package io

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	conc = 100               // concurrent client count
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
	return s.Push("pong",
		&testdata.Pong{
			Content:  data.Content,
			Sequence: data.Sequence + 1,
		})
}

func server() {
	components := &component.Components{}
	components.Register(&TestHandler{})
	nano.Listen(addr,
		nano.WithComponents(components),
		//nano.WithDebugMode(),
	)
}

func client(id int, ttl time.Duration, done *sync.WaitGroup) {
	c := NewConnector()
	ready := make(chan struct{})
	pingSeq := int64(-1) // ping sequence is 1,3,5,7,...
	pongSeq := int64(0)  // pong sequence should be 2,4,6,8,...
	quit := time.After(ttl)

	c.OnConnected(func() {
		ready <- struct{}{}
	})
	c.On("pong", func(data interface{}) {
		res := new(testdata.Pong)
		if err := deserialize(data.([]byte), res); err != nil {
			panic(err)
		}
		pongSeq = res.Sequence
	})
	if err := c.Start(addr); err != nil {
		panic(err)
	}
	<-ready

LOOP:
	for {
		select {
		case <-quit:
			c.Close()
			break LOOP
		default:
			pingSeq += 2
			if err := c.Notify("TestHandler.Ping",
				&testdata.Ping{
					Content:  "hello",
					Sequence: pingSeq,
				}); err != nil {
				panic(err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	fmt.Printf("client %v done with ping_seq=%v pong_seq=%v\n", id, pingSeq, pongSeq)
	done.Done()
}

func TestIO(t *testing.T) {
	go server()

	// wait server startup
	time.Sleep(time.Second)
	t.Log("server started")

	var wg sync.WaitGroup
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go client(i, 10*time.Second, &wg)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)
	select {
	case <-interrupt:
	case <-done:
	}

	t.Log("exit")
}
