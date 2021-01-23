package gate

import "github.com/nano-kit/go-nano/component"

var (
	// Services in master server
	Services = &component.Components{}

	bindService = newBindService()
)

func init() {
	Services.Register(bindService)
}
