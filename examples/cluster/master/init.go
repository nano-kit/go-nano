package master

import (
	"github.com/nano-kit/go-nano/component"
	"github.com/nano-kit/go-nano/session"
)

var (
	// Services in master server
	Services = &component.Components{}

	// Topic service
	topicService = newTopicService()
	// ... other services
)

func init() {
	Services.Register(topicService)
}

func OnSessionClosed(s *session.Session) {
	topicService.userDisconnected(s)
}
