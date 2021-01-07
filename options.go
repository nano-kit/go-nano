package nano

import (
	"net/http"
	"time"

	"github.com/aclisp/go-nano/cluster"
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/internal/env"
	"github.com/aclisp/go-nano/internal/log"
	"github.com/aclisp/go-nano/internal/message"
	"github.com/aclisp/go-nano/pipeline"
	"github.com/aclisp/go-nano/serialize"
	"google.golang.org/grpc"
)

// Option is a function to set cluster options
type Option func(*cluster.Options)

// WithPipeline sets processing pipelines
func WithPipeline(pipeline pipeline.Pipeline) Option {
	return func(opt *cluster.Options) {
		opt.Pipeline = pipeline
	}
}

// WithRegistryAddr sets the registry address option, it will be the service address of
// master node and an advertise address which cluster member to connect
func WithRegistryAddr(addr string, regInterval ...time.Duration) Option {
	return func(opt *cluster.Options) {
		opt.RegistryAddr = addr
		if len(regInterval) > 0 {
			opt.RegisterInterval = regInterval[0]
		}
	}
}

// WithGateAddr sets the listen address which is used by client to establish connection.
func WithGateAddr(addr string) Option {
	return func(opt *cluster.Options) {
		opt.GateAddr = addr
	}
}

// WithMaster sets the option to indicate whether the current node is master node
func WithMaster() Option {
	return func(opt *cluster.Options) {
		opt.IsMaster = true
	}
}

// WithGrpcOptions sets the grpc dial options
func WithGrpcOptions(opts ...grpc.DialOption) Option {
	return func(_ *cluster.Options) {
		env.GrpcOptions = append(env.GrpcOptions, opts...)
	}
}

// WithComponents sets the Components
func WithComponents(components *component.Components) Option {
	return func(opt *cluster.Options) {
		opt.Components = components
	}
}

// WithHeartbeatInterval sets Heartbeat time interval
func WithHeartbeatInterval(d time.Duration) Option {
	return func(_ *cluster.Options) {
		env.Heartbeat = d
	}
}

// WithCheckOriginFunc sets the function that check `Origin` in http headers
func WithCheckOriginFunc(fn func(*http.Request) bool) Option {
	return func(opt *cluster.Options) {
		opt.CheckOrigin = fn
	}
}

// WithDebugMode let 'nano' to run under Debug mode.
func WithDebugMode() Option {
	return func(_ *cluster.Options) {
		env.Debug = true
	}
}

// WithDictionary sets routes map
func WithDictionary(dict map[string]uint16) Option {
	return func(_ *cluster.Options) {
		message.SetDictionary(dict)
	}
}

// WithWSPath sets websocket URI path, effective when WebSocket is enabled
func WithWSPath(path string) Option {
	return func(opt *cluster.Options) {
		opt.WSPath = path
	}
}

// WithTimerPrecision sets the ticker precision, and time precision can not less
// than a Millisecond, and can not change after application running. The default
// precision is time.Second
func WithTimerPrecision(precision time.Duration) Option {
	if precision < time.Millisecond {
		panic("time precision can not less than a Millisecond")
	}
	return func(_ *cluster.Options) {
		env.TimerPrecision = precision
	}
}

// WithSerializer customizes application serializer, which automatically Marshal
// and UnMarshal handler payload
func WithSerializer(serializer serialize.Serializer) Option {
	return func(opt *cluster.Options) {
		env.Serializer = serializer
	}
}

// WithLabel sets the current node label in cluster
func WithLabel(label string) Option {
	return func(opt *cluster.Options) {
		opt.Label = label
	}
}

// WithIsWebsocket indicates whether current node WebSocket is enabled
func WithIsWebsocket(enableWs bool) Option {
	return func(opt *cluster.Options) {
		opt.IsWebsocket = enableWs
	}
}

// WithTSLConfig sets the `key` and `certificate` of TSL
func WithTSLConfig(certificate, key string) Option {
	return func(opt *cluster.Options) {
		opt.TSLCertificate = certificate
		opt.TSLKey = key
	}
}

// WithLogger overrides the default logger
func WithLogger(l log.Logger) Option {
	return func(opt *cluster.Options) {
		log.SetLogger(l)
	}
}

// WithHandshakeValidator sets the function that Verify `handshake` data
func WithHandshakeValidator(fn func([]byte) error) Option {
	return func(opt *cluster.Options) {
		env.HandshakeValidator = fn
	}
}

// WithHTTPHandler sets a http handler that shares with WebSocket server
func WithHTTPHandler(pattern string, handler http.Handler) Option {
	return func(opt *cluster.Options) {
		opt.ServeMux.Handle(pattern, handler)
	}
}
