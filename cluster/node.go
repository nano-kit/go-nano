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
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nano-kit/go-nano/cluster/clusterpb"
	"github.com/nano-kit/go-nano/component"
	"github.com/nano-kit/go-nano/internal/log"
	"github.com/nano-kit/go-nano/internal/message"
	"github.com/nano-kit/go-nano/pipeline"
	"github.com/nano-kit/go-nano/scheduler"
	"github.com/nano-kit/go-nano/service"
	"github.com/nano-kit/go-nano/session"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// Options contains some configurations for current node
type Options struct {
	Pipeline         pipeline.Pipeline
	IsMaster         bool
	RegistryAddr     string
	RegisterInterval time.Duration
	GateAddr         string
	Components       *component.Components
	Label            string
	MonitorAddr      string

	WebsocketOptions
}

// WebsocketOptions contains WebSocket related configurations
type WebsocketOptions struct {
	IsWebsocket    bool
	TSLCertificate string
	TSLKey         string
	WSPath         string                   // WebSocket path (eg: ws://127.0.0.1/WSPath)
	ServeMux       *http.ServeMux           // do not rely on http.DefaultServeMux, use a private mux
	CheckOrigin    func(*http.Request) bool // check origin when websocket enabled
}

// NewOptions creates Options
func NewOptions() Options {
	return Options{
		Components: &component.Components{},
		WebsocketOptions: WebsocketOptions{
			ServeMux:    http.NewServeMux(),
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
	}
}

// Node represents a node in nano cluster, which will contains a group of services.
// All services will register to cluster and messages will be forwarded to the node
// which provides respective service
type Node struct {
	Options            // current node options
	ServiceAddr string // current server service address

	cluster   *cluster
	handler   *LocalHandler
	rpcServer *grpc.Server
	rpcClient *rpcClient

	mu       sync.RWMutex
	sessions map[service.SID]*session.Session
}

func validateListenAddrWithExplicitPort(addr string) error {
	if addr == "" {
		return errors.New("address cannot be empty")
	}

	if _, port, err := net.SplitHostPort(addr); err != nil {
		return err
	} else if port == "" || port == "0" {
		return errors.New("port number cannot be automatically chosen")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	listener.Close()
	return nil
}

// Startup starts the node
func (n *Node) Startup() error {
	if err := validateListenAddrWithExplicitPort(n.ServiceAddr); err != nil {
		return fmt.Errorf("invalid node service address: %v", err)
	}

	n.sessions = map[service.SID]*session.Session{}
	n.cluster = newCluster(n)
	n.handler = NewHandler(n, n.Pipeline)
	components := n.Components.List()
	for _, c := range components {
		err := n.handler.register(c.Comp, c.Opts)
		if err != nil {
			return err
		}
	}

	cache()
	n.adjustOpenFilesLimit()
	if err := n.initNode(); err != nil {
		return err
	}

	// Initialize all components
	for _, c := range components {
		c.Comp.Init()
	}
	for _, c := range components {
		c.Comp.AfterInit()
	}

	if n.GateAddr != "" {
		go func() {
			if n.IsWebsocket {
				if len(n.TSLCertificate) != 0 {
					n.listenAndServeWSTLS()
				} else {
					n.listenAndServeWS()
				}
			} else {
				n.listenAndServe()
			}
		}()
		n.waitForGate(time.Second)
	}

	n.startMonitor()
	scheduler.Repeat(n.removeStaleSession, 67*time.Second)
	return nil
}

func (n *Node) waitForGate(timeout time.Duration) {
	begin := time.Now()
	for time.Since(begin) < timeout {
		if conn, err := net.Dial("tcp", n.GateAddr); err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				time.Sleep(10 * time.Millisecond)
				continue
			}
		} else {
			conn.Close()
		}
		break
	}
}

func (n *Node) adjustOpenFilesLimit() {
	const (
		MinReservedFDs = 64
		MaxClients     = 10000
	)
	var (
		err      error
		maxfiles uint64 = MaxClients + MinReservedFDs
		limit    unix.Rlimit
	)

	if err = unix.Getrlimit(unix.RLIMIT_NOFILE, &limit); err != nil {
		log.Print("unable to obtain the current NOFILE limit", err)
		return
	}

	oldlimit := limit.Cur

	// Set the max number of files if the current limit is not enough for our needs
	bestlimit := maxfiles
	for bestlimit > oldlimit {
		var decrStep uint64 = 16

		limit.Cur = bestlimit
		limit.Max = bestlimit
		if err = unix.Setrlimit(unix.RLIMIT_NOFILE, &limit); err == nil {
			break
		}

		// We failed to set file limit to 'bestlimit'. Try with a
		// smaller limit decrementing by a few FDs per iteration.
		if bestlimit < decrStep {
			break
		}
		bestlimit -= decrStep
	}

	// Assume that the limit we get initially is still valid if
	// our last try was even lower.
	if bestlimit < oldlimit {
		bestlimit = oldlimit
	}

	if bestlimit < maxfiles {
		oldMaxclients := MaxClients
		maxclients := bestlimit - MinReservedFDs
		// maxclients is unsigned so may overflow: in order
		// to check if maxclients is now logically less than 1
		// we test indirectly via bestlimit.
		if bestlimit <= MinReservedFDs {
			log.Fatalf("Your current 'ulimit -n' of %v is not enough for the server to start. "+
				"Please increase your open file limit to at least %v. Exiting.", oldlimit, maxfiles)
		}
		log.Printf("You requested maxclients of %v requiring at least %v max file descriptors.", oldMaxclients, maxfiles)
		log.Printf("Server can't set maximum open files to %v because of OS error: %v", maxfiles, err)
		log.Printf("Current maximum open files is %v. "+
			"maxclients has been reduced to %v to compensate for low ulimit. "+
			"If you need higher maxclients increase 'ulimit -n'.", bestlimit, maxclients)
	} else {
		log.Printf("increased maximum number of open files to %v (it was originally set to %v)", maxfiles, oldlimit)
	}
}

// Handler returns node's local handler
func (n *Node) Handler() *LocalHandler {
	return n.handler
}

func (n *Node) initNode() error {
	// Current node is not master server and does not contains master
	// address, so running in singleton mode
	if !n.IsMaster && n.RegistryAddr == "" {
		return nil
	}

	listener, err := net.Listen("tcp", n.ServiceAddr)
	if err != nil {
		return err
	}

	// Initialize the gRPC server and register service
	n.rpcServer = grpc.NewServer()
	n.rpcClient = newRPCClient()
	scheduler.Repeat(n.shrinkRPCClient, 61*time.Second)
	clusterpb.RegisterMemberServer(n.rpcServer, n)

	go func() {
		err := n.rpcServer.Serve(listener)
		if err != nil {
			log.Fatalf("start current node failed: %v", err)
		}
	}()

	if n.IsMaster {
		clusterpb.RegisterMasterServer(n.rpcServer, n.cluster)
		member := &Member{
			IsMaster: true,
			MemberInfo: &clusterpb.MemberInfo{
				Label:       n.Label,
				ServiceAddr: n.ServiceAddr,
				Services:    n.handler.LocalService(),
			},
		}
		n.cluster.members = append(n.cluster.members, member)
		n.cluster.setRPCClient(n.rpcClient)
	} else {
		pool, err := n.rpcClient.getConnPool(n.RegistryAddr)
		if err != nil {
			return err
		}
		client := clusterpb.NewMasterClient(pool.Get())
		request := &clusterpb.RegisterRequest{
			MemberInfo: &clusterpb.MemberInfo{
				Label:       n.Label,
				ServiceAddr: n.ServiceAddr,
				Services:    n.handler.LocalService(),
			},
		}
		for {
			resp, err := client.Register(context.Background(), request)
			if err == nil {
				n.handler.initRemoteService(resp.Members)
				n.cluster.initMembers(resp.Members)
				break
			}

			log.Print("register current node to cluster failed", err, "and will unregister then retry in", n.RegisterInterval.String())
			time.Sleep(n.RegisterInterval)
			n.unregister()
		}
	}

	return nil
}

// Shutdown all components registered by application, that
// call by reverse order against register
func (n *Node) Shutdown() {
	// reverse call `BeforeShutdown` hooks
	components := n.Components.List()
	length := len(components)
	for i := length - 1; i >= 0; i-- {
		components[i].Comp.BeforeShutdown()
	}

	// reverse call `Shutdown` hooks
	for i := length - 1; i >= 0; i-- {
		components[i].Comp.Shutdown()
	}

	if !n.IsMaster && n.RegistryAddr != "" {
		n.unregister()
	}

	if n.rpcServer != nil {
		n.rpcServer.GracefulStop()
	}
}

func (n *Node) unregister() error {
	pool, err := n.rpcClient.getConnPool(n.RegistryAddr)
	if err != nil {
		log.Print("retrieve master address error", err)
		return err
	}
	client := clusterpb.NewMasterClient(pool.Get())
	request := &clusterpb.UnregisterRequest{
		ServiceAddr: n.ServiceAddr,
	}
	_, err = client.Unregister(context.Background(), request)
	if err != nil {
		log.Print("unregister current node failed", err)
		return err
	}
	return nil
}

// Enable current server accept connection
func (n *Node) listenAndServe() {
	listener, err := net.Listen("tcp", n.GateAddr)
	if err != nil {
		log.Fatal(err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err.Error())
			continue
		}

		go n.handler.handle(conn)
	}
}

func (n *Node) listenAndServeWS() {
	n.setupWSHandler()

	if err := http.ListenAndServe(n.GateAddr, n.ServeMux); err != nil {
		log.Fatal(err.Error())
	}
}

func (n *Node) listenAndServeWSTLS() {
	n.setupWSHandler()

	if err := http.ListenAndServeTLS(n.GateAddr, n.TSLCertificate, n.TSLKey, n.ServeMux); err != nil {
		log.Fatal(err.Error())
	}
}

func (n *Node) setupWSHandler() {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     n.CheckOrigin,
	}

	n.ServeMux.HandleFunc("/"+strings.TrimPrefix(n.WSPath, "/"), func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade failure, URI=%s, Error=%s", r.RequestURI, err.Error())
			return
		}

		n.handler.handleWS(conn)
	})
}

func (n *Node) storeSession(s *session.Session) {
	n.mu.Lock()
	n.sessions[s.ID()] = s
	n.mu.Unlock()
}

func (n *Node) removeSession(s *session.Session) {
	n.mu.Lock()
	delete(n.sessions, s.ID())
	n.mu.Unlock()
}

func (n *Node) findSession(sid service.SID) *session.Session {
	n.mu.RLock()
	s := n.sessions[sid]
	n.mu.RUnlock()
	return s
}

func (n *Node) findOrCreateSession(sid service.SID, gateAddr string) (*session.Session, error) {
	n.mu.RLock()
	s, found := n.sessions[sid]
	n.mu.RUnlock()
	if !found {
		conns, err := n.rpcClient.getConnPool(gateAddr)
		if err != nil {
			return nil, err
		}
		ac := &acceptor{
			sid:        sid,
			gateClient: clusterpb.NewMemberClient(conns.Get()),
			rpcHandler: n.handler.remoteProcess,
			gateAddr:   gateAddr,
		}
		s = session.NewWith(sid, ac)
		ac.session = s
		n.mu.Lock()
		n.sessions[sid] = s
		n.mu.Unlock()
	}
	return s, nil
}

// HandleRequest implements the MemberServer interface
func (n *Node) HandleRequest(_ context.Context, req *clusterpb.RequestMessage) (*clusterpb.MemberHandleResponse, error) {
	handler, found := n.handler.localHandlers[req.Route]
	if !found {
		return nil, fmt.Errorf("service not found in current node: %v", req.Route)
	}
	s, err := n.findOrCreateSession(service.SID(req.SessionId), req.GateAddr)
	if err != nil {
		return nil, err
	}
	msg := &message.Message{
		Type:  message.Request,
		ID:    req.Id,
		Route: req.Route,
		Data:  req.Data,
	}
	n.handler.localProcess(handler, req.Id, s, msg)
	s.AdvanceLastTime()
	return &clusterpb.MemberHandleResponse{}, nil
}

// HandleNotify implements the MemberServer interface
func (n *Node) HandleNotify(_ context.Context, req *clusterpb.NotifyMessage) (*clusterpb.MemberHandleResponse, error) {
	handler, found := n.handler.localHandlers[req.Route]
	if !found {
		return nil, fmt.Errorf("service not found in current node: %v", req.Route)
	}
	s, err := n.findOrCreateSession(service.SID(req.SessionId), req.GateAddr)
	if err != nil {
		return nil, err
	}
	msg := &message.Message{
		Type:  message.Notify,
		Route: req.Route,
		Data:  req.Data,
	}
	n.handler.localProcess(handler, 0, s, msg)
	s.AdvanceLastTime()
	return &clusterpb.MemberHandleResponse{}, nil
}

// HandlePush implements the MemberServer interface
func (n *Node) HandlePush(_ context.Context, req *clusterpb.PushMessage) (*clusterpb.MemberHandleResponse, error) {
	s := n.findSession(service.SID(req.SessionId))
	if s == nil {
		return &clusterpb.MemberHandleResponse{}, fmt.Errorf("session not found: %v", req.SessionId)
	}
	return &clusterpb.MemberHandleResponse{}, s.Push(req.Route, req.Data)
}

// HandleResponse implements the MemberServer interface
func (n *Node) HandleResponse(_ context.Context, req *clusterpb.ResponseMessage) (*clusterpb.MemberHandleResponse, error) {
	s := n.findSession(service.SID(req.SessionId))
	if s == nil {
		return &clusterpb.MemberHandleResponse{}, fmt.Errorf("session not found: %v", req.SessionId)
	}
	return &clusterpb.MemberHandleResponse{}, s.ResponseMID(req.Id, req.Data)
}

// NewMember implements the MemberServer interface
func (n *Node) NewMember(_ context.Context, req *clusterpb.NewMemberRequest) (*clusterpb.NewMemberResponse, error) {
	n.handler.addRemoteService(req.MemberInfo)
	n.cluster.addMember(req.MemberInfo)
	return &clusterpb.NewMemberResponse{}, nil
}

// DelMember implements the MemberServer interface
func (n *Node) DelMember(_ context.Context, req *clusterpb.DelMemberRequest) (*clusterpb.DelMemberResponse, error) {
	n.handler.delMember(req.ServiceAddr)
	n.cluster.delMember(req.ServiceAddr)
	return &clusterpb.DelMemberResponse{}, nil
}

// SessionClosed implements the MemberServer interface
func (n *Node) SessionClosed(_ context.Context, req *clusterpb.SessionClosedRequest) (*clusterpb.SessionClosedResponse, error) {
	sid := service.SID(req.SessionId)
	n.mu.Lock()
	s, found := n.sessions[sid]
	delete(n.sessions, sid)
	n.mu.Unlock()
	if found {
		scheduler.Run(func() { session.Lifetime.Close(s) })
	}
	return &clusterpb.SessionClosedResponse{}, nil
}

// CloseSession implements the MemberServer interface
func (n *Node) CloseSession(_ context.Context, req *clusterpb.CloseSessionRequest) (*clusterpb.CloseSessionResponse, error) {
	sid := service.SID(req.SessionId)
	n.mu.Lock()
	s, found := n.sessions[sid]
	delete(n.sessions, sid)
	n.mu.Unlock()
	if found {
		s.Close()
	}
	return &clusterpb.CloseSessionResponse{}, nil
}
