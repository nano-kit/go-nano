package chat

import (
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/session"
)

var (
	// Services in master server
	Services = &component.Components{}

	roomService = newRoomService()
)

func init() {
	Services.Register(roomService)
}

func OnSessionClosed(s *session.Session) {
	roomService.userDisconnected(s)
}
