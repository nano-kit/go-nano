package logic

import (
	"log"
	"strconv"

	"github.com/nano-kit/go-nano/component"
	"github.com/nano-kit/go-nano/examples/demo/tadpole/logic/protocol"
	"github.com/nano-kit/go-nano/session"
)

// Manager component
type Manager struct {
	component.Base
}

// NewManager returns  a new manager instance
func NewManager() *Manager {
	return &Manager{}
}

// Login handler was used to guest login
func (m *Manager) Login(s *session.Session, msg *protocol.JoyLoginRequest) error {
	log.Println(msg)
	id := int64(s.ID())
	uid := strconv.FormatInt(id, 10)
	s.Bind(uid)
	return s.Response(protocol.LoginResponse{
		Status: protocol.LoginStatusSucc,
		ID:     id,
	})
}
