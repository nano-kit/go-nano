package cluster

import (
	"context"
	"net"

	"github.com/nano-kit/go-nano/cluster/clusterpb"
	"github.com/nano-kit/go-nano/internal/message"
	"github.com/nano-kit/go-nano/service"
	"github.com/nano-kit/go-nano/session"
)

type acceptor struct {
	sid        service.SID
	gateClient clusterpb.MemberClient
	session    *session.Session
	lastMid    uint64
	rpcHandler rpcHandler
	gateAddr   string
}

// Push implements the session.NetworkEntity interface
func (a *acceptor) Push(route string, v interface{}) error {
	// TODO: buffer
	data, err := message.Serialize(v)
	if err != nil {
		return err
	}
	request := &clusterpb.PushMessage{
		SessionId: int64(a.sid),
		Route:     route,
		Data:      data,
	}
	_, err = a.gateClient.HandlePush(context.Background(), request)
	return err
}

// Notify implements the session.NetworkEntity interface
func (a *acceptor) Notify(route string, v interface{}) error {
	// TODO: buffer
	data, err := message.Serialize(v)
	if err != nil {
		return err
	}
	msg := &message.Message{
		Type:  message.Notify,
		Route: route,
		Data:  data,
	}
	a.rpcHandler(a.session, msg, true)
	return nil
}

// LastMid implements the session.NetworkEntity interface
func (a *acceptor) LastMid() uint64 {
	return a.lastMid
}

// Response implements the session.NetworkEntity interface
func (a *acceptor) Response(v interface{}) error {
	return a.ResponseMid(a.lastMid, v)
}

// ResponseMid implements the session.NetworkEntity interface
func (a *acceptor) ResponseMid(mid uint64, v interface{}) error {
	// TODO: buffer
	data, err := message.Serialize(v)
	if err != nil {
		return err
	}
	request := &clusterpb.ResponseMessage{
		SessionId: int64(a.sid),
		Id:        mid,
		Data:      data,
	}
	_, err = a.gateClient.HandleResponse(context.Background(), request)
	return err
}

// Close implements the session.NetworkEntity interface
func (a *acceptor) Close() error {
	// TODO: buffer
	request := &clusterpb.CloseSessionRequest{
		SessionId: int64(a.sid),
	}
	_, err := a.gateClient.CloseSession(context.Background(), request)
	return err
}

// RemoteAddr implements the session.NetworkEntity interface
func (a *acceptor) RemoteAddr() net.Addr {
	return acceptorRemoteAddr{network: "grpc", address: a.gateAddr}
}

type acceptorRemoteAddr struct {
	network string
	address string
}

func (a acceptorRemoteAddr) Network() string { return a.network }

func (a acceptorRemoteAddr) String() string { return a.address }
