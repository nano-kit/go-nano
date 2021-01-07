// Copyright (c) nano Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aclisp/go-nano/cluster/clusterpb"
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/internal/codec"
	"github.com/aclisp/go-nano/internal/env"
	"github.com/aclisp/go-nano/internal/log"
	"github.com/aclisp/go-nano/internal/message"
	"github.com/aclisp/go-nano/internal/packet"
	"github.com/aclisp/go-nano/pipeline"
	"github.com/aclisp/go-nano/scheduler"
	"github.com/aclisp/go-nano/session"
	"github.com/gorilla/websocket"
)

var (
	// cached serialized data
	hrd []byte // handshake response data
	hbd []byte // heartbeat packet data
)

type rpcHandler func(session *session.Session, msg *message.Message, noCopy bool)

func cache() {
	data, err := json.Marshal(map[string]interface{}{
		"code": 200,
		"sys":  map[string]float64{"heartbeat": env.Heartbeat.Seconds()},
	})
	if err != nil {
		panic(err)
	}

	hrd, err = codec.Encode(packet.Handshake, data)
	if err != nil {
		panic(err)
	}

	hbd, err = codec.Encode(packet.Heartbeat, nil)
	if err != nil {
		panic(err)
	}
}

// LocalHandler is the container for all local registered components
type LocalHandler struct {
	localServices map[string]*component.Service // all registered service
	localHandlers map[string]*component.Handler // all handler method

	mu             sync.RWMutex
	remoteServices map[string][]*clusterpb.MemberInfo

	pipeline    pipeline.Pipeline
	currentNode *Node
}

// NewHandler creates a LocalHandler
func NewHandler(currentNode *Node, pipeline pipeline.Pipeline) *LocalHandler {
	h := &LocalHandler{
		localServices:  make(map[string]*component.Service),
		localHandlers:  make(map[string]*component.Handler),
		remoteServices: map[string][]*clusterpb.MemberInfo{},
		pipeline:       pipeline,
		currentNode:    currentNode,
	}

	return h
}

func (h *LocalHandler) register(comp component.Component, opts []component.Option) error {
	s := component.NewService(comp, opts)

	if _, ok := h.localServices[s.Name]; ok {
		return fmt.Errorf("handler: service already defined: %s", s.Name)
	}

	if err := s.ExtractHandler(); err != nil {
		return err
	}

	// register all localHandlers
	h.localServices[s.Name] = s
	for name, handler := range s.Handlers {
		n := fmt.Sprintf("%s.%s", s.Name, name)
		log.Print("register local handler", n)
		h.localHandlers[n] = handler
	}
	return nil
}

func (h *LocalHandler) initRemoteService(members []*clusterpb.MemberInfo) {
	for _, m := range members {
		h.addRemoteService(m)
	}
}

func (h *LocalHandler) addRemoteService(member *clusterpb.MemberInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, s := range member.Services {
		log.Print("register remote service", s)
		h.remoteServices[s] = append(h.remoteServices[s], member)
	}
}

func (h *LocalHandler) delMember(addr string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for name, members := range h.remoteServices {
		for i, maddr := range members {
			if addr == maddr.ServiceAddr {
				if i == len(members)-1 {
					members = members[:i]
				} else {
					members = append(members[:i], members[i+1:]...)
				}
			}
		}
		if len(members) == 0 {
			delete(h.remoteServices, name)
		} else {
			h.remoteServices[name] = members
		}
	}
}

// LocalService returns a sorted local service names
func (h *LocalHandler) LocalService() []string {
	var result []string
	for service := range h.localServices {
		result = append(result, service)
	}
	sort.Strings(result)
	return result
}

// CompInfo is the component information used by the node monitor
type CompInfo struct {
	Name         string
	ReceiverType string
	HandlerType  string
	IsRawArg     bool
	Scheduler    string
}

// Components show a sorted list of local components for the node monitor
func (h *LocalHandler) Components() []CompInfo {
	var result []CompInfo
	for _, service := range h.LocalService() {
		s := h.localServices[service]
		for _, handler := range s.SortedHandlers() {
			m := s.Handlers[handler]
			result = append(result, CompInfo{
				Name:         fmt.Sprintf("%s.%s", service, handler),
				ReceiverType: s.Type.String(),
				HandlerType:  m.Type.String(),
				IsRawArg:     m.IsRawArg,
				Scheduler:    s.SchedName,
			})
		}
	}
	return result
}

// RemoteService returns a sorted remote service names
func (h *LocalHandler) RemoteService() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []string
	for service := range h.remoteServices {
		result = append(result, service)
	}
	sort.Strings(result)
	return result
}

// RemoteInfo is the remote component information used by the node monitor
type RemoteInfo struct {
	Name string
	*clusterpb.MemberInfo
}

// Remotes show a sorted remote components list for the node monitor
func (h *LocalHandler) Remotes() []RemoteInfo {
	var result []RemoteInfo
	for _, remote := range h.RemoteService() {
		h.mu.RLock()
		s := h.remoteServices[remote]
		for _, m := range s {
			result = append(result, RemoteInfo{
				Name:       remote,
				MemberInfo: m,
			})
		}
		h.mu.RUnlock()
	}
	return result
}

func (h *LocalHandler) handle(conn net.Conn) {
	// create a client agent and startup write gorontine
	agent := newAgent(conn, h.pipeline, h.remoteProcess)
	h.currentNode.storeSession(agent.session)

	// startup write goroutine
	go agent.write()

	if env.Debug {
		log.Printf("new session established: %s", agent.String())
	}

	// guarantee agent related resource be destroyed
	defer func() {
		agent.notifySessionClosed(h.currentNode.rpcClient, h.currentNode.cluster.remoteAddrs())
		h.currentNode.removeSession(agent.session)
		agent.Close()
		if env.Debug {
			log.Printf("session read goroutine exit, SessionID=%d, UID=%d", agent.session.ID(), agent.session.UID())
		}
	}()

	// read loop
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("read message error: %s, session will be closed immediately", err.Error())
			return
		}

		// TODO(warning): decoder use slice for performance, packet data should be copy before next Decode
		packets, err := agent.decoder.Decode(buf[:n])
		if err != nil {
			log.Print(err.Error())
			return
		}

		if len(packets) < 1 {
			continue
		}

		// process all packet
		for i := range packets {
			if err := h.processPacket(agent, packets[i]); err != nil {
				log.Print(err.Error())
				return
			}
		}
	}
}

func (h *LocalHandler) processPacket(agent *agent, p *packet.Packet) error {
	switch p.Type {
	case packet.Handshake:
		if err := env.HandshakeValidator(p.Data); err != nil {
			return err
		}

		if _, err := agent.conn.Write(hrd); err != nil {
			return err
		}

		agent.setStatus(statusHandshake)
		if env.Debug {
			log.Printf("session handshake Id=%d, Remote=%s", agent.session.ID(), agent.conn.RemoteAddr())
		}

	case packet.HandshakeAck:
		agent.setStatus(statusWorking)
		if env.Debug {
			log.Printf("receive handshake ACK Id=%d, Remote=%s", agent.session.ID(), agent.conn.RemoteAddr())
		}

	case packet.Data:
		if agent.status() < statusWorking {
			return fmt.Errorf("receive data on socket which not yet ACK, session will be closed immediately, remote=%s",
				agent.conn.RemoteAddr().String())
		}

		msg, err := message.Decode(p.Data)
		if err != nil {
			return err
		}
		h.processMessage(agent, msg)

	case packet.Heartbeat:
	}

	now := time.Now().Unix()
	atomic.StoreInt64(&agent.lastAt, now)
	agent.session.AdvanceLastTimeTo(now)
	return nil
}

func (h *LocalHandler) findMembers(service string) []*clusterpb.MemberInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.remoteServices[service]
}

func (h *LocalHandler) remoteProcess(session *session.Session, msg *message.Message, noCopy bool) {
	index := strings.LastIndex(msg.Route, ".")
	if index < 0 {
		log.Printf("nano/handler: invalid route %s", msg.Route)
		return
	}

	service := msg.Route[:index]
	members := h.findMembers(service)
	if len(members) == 0 {
		log.Printf("nano/handler: %s not found (forgot registered?)", msg.Route)
		return
	}

	// Select a remote service address
	// 1. Use the service address directly if the router contains binding item
	// 2. Select a remote service address randomly and bind to router
	var remoteAddr string
	if addr, found := session.Router().Find(service); found {
		remoteAddr = addr
	} else {
		remoteAddr = members[rand.Intn(len(members))].ServiceAddr
		session.Router().Bind(service, remoteAddr)
	}
	pool, err := h.currentNode.rpcClient.getConnPool(remoteAddr)
	if err != nil {
		log.Print(err)
		return
	}
	var data = msg.Data
	if !noCopy && len(msg.Data) > 0 {
		data = make([]byte, len(msg.Data))
		copy(data, msg.Data)
	}

	// Retrieve gate address and session id
	gateAddr := h.currentNode.ServiceAddr
	sessionID := session.ID()
	switch v := session.NetworkEntity().(type) {
	case *acceptor:
		gateAddr = v.gateAddr
		sessionID = v.sid
	}

	client := clusterpb.NewMemberClient(pool.Get())
	switch msg.Type {
	case message.Request:
		request := &clusterpb.RequestMessage{
			GateAddr:  gateAddr,
			SessionId: sessionID,
			Id:        msg.ID,
			Route:     msg.Route,
			Data:      data,
		}
		_, err = client.HandleRequest(context.Background(), request)
	case message.Notify:
		request := &clusterpb.NotifyMessage{
			GateAddr:  gateAddr,
			SessionId: sessionID,
			Route:     msg.Route,
			Data:      data,
		}
		_, err = client.HandleNotify(context.Background(), request)
	}
	if err != nil {
		log.Printf("process remote message to %s error: %+v", msg.Route, err)
	}
}

func (h *LocalHandler) processMessage(agent *agent, msg *message.Message) {
	var lastMid uint64
	switch msg.Type {
	case message.Request:
		lastMid = msg.ID
	case message.Notify:
		lastMid = 0
	default:
		log.Print("invalid message type: " + msg.Type.String())
		return
	}

	handler, found := h.localHandlers[msg.Route]
	if !found {
		h.remoteProcess(agent.session, msg, false)
	} else {
		h.localProcess(handler, lastMid, agent.session, msg)
	}
}

func (h *LocalHandler) handleWS(conn *websocket.Conn) {
	c, err := newWSConn(conn)
	if err != nil {
		log.Print(err)
		return
	}
	go h.handle(c)
}

func (h *LocalHandler) localProcess(handler *component.Handler, lastMid uint64, session *session.Session, msg *message.Message) {
	if pipe := h.pipeline; pipe != nil {
		err := pipe.Inbound().Process(session, msg)
		if err != nil {
			log.Print("pipeline process failed: " + err.Error())
			return
		}
	}

	var payload = msg.Data
	var data interface{}
	if handler.IsRawArg {
		if len(payload) > 0 {
			temp := make([]byte, len(payload))
			copy(temp, payload)
			payload = temp
		}
		data = payload
	} else {
		data = reflect.New(handler.Type.Elem()).Interface()
		err := env.Serializer.Unmarshal(payload, data)
		if err != nil {
			log.Printf("deserialize to %T failed: %+v (%v)", data, err, payload)
			return
		}
	}

	if env.Debug {
		log.Printf("UID=%d, Message={%s}, Data=%+v", session.UID(), msg.String(), data)
	}

	args := []reflect.Value{handler.Receiver, reflect.ValueOf(session), reflect.ValueOf(data)}
	task := func() {
		switch v := session.NetworkEntity().(type) {
		case *agent:
			v.lastMid = lastMid
		case *acceptor:
			v.lastMid = lastMid
		}

		result := handler.Method.Func.Call(args)
		if len(result) > 0 {
			if err := result[0].Interface(); err != nil {
				log.Printf("service %s error: %+v", msg.Route, err)
			}
		}
	}

	index := strings.LastIndex(msg.Route, ".")
	if index < 0 {
		log.Printf("nano/handler: invalid route %s", msg.Route)
		return
	}

	// A message can be dispatch to global thread or a user customized thread
	service := msg.Route[:index]
	if s, found := h.localServices[service]; found && s.SchedName != "" {
		sched := session.Value(s.SchedName)
		if sched == nil {
			log.Printf("nano/handler: cannot found `schedular.LocalScheduler` by %s", s.SchedName)
			return
		}

		local, ok := sched.(scheduler.LocalScheduler)
		if !ok {
			log.Printf("nano/handler: Type %T does not implement the `schedular.LocalScheduler` interface",
				sched)
			return
		}
		local.Schedule(task)
	} else {
		scheduler.PushTask(task)
	}
}
