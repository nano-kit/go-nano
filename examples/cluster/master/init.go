package master

import (
	"github.com/lonng/nano/component"
	"github.com/lonng/nano/session"
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
