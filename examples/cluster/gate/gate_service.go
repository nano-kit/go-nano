package gate

import (
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/examples/cluster/protocol"
	"github.com/aclisp/go-nano/session"
	"github.com/pingcap/errors"
)

type BindService struct {
	component.Base
	nextGateUID int64
}

func newBindService() *BindService {
	return &BindService{}
}

type (
	LoginRequest struct {
		Nickname string `json:"nickname"`
	}
	LoginResponse struct {
		Code int `json:"code"`
	}
)

func (bs *BindService) Login(s *session.Session, msg *LoginRequest) error {
	bs.nextGateUID++
	uid := bs.nextGateUID
	request := &protocol.NewUserRequest{
		Nickname: msg.Nickname,
		GateUID:  uid,
	}
	if err := s.RPC("TopicService.NewUser", request); err != nil {
		return errors.Trace(err)
	}
	return s.Response(&LoginResponse{})
}

func (bs *BindService) BindChatServer(s *session.Session, msg []byte) error {
	return errors.Errorf("not implement")
}
