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

// Package env represents the environment of the current process, includes
// work path and config path etc.
package env

import (
	"time"

	"github.com/nano-kit/go-nano/serialize"
	"github.com/nano-kit/go-nano/serialize/protobuf"
	"google.golang.org/grpc"
)

var (
	Wd                 string             // working path
	GateID             uint16             // gate id
	Die                chan bool          // wait for end application
	Heartbeat          time.Duration      // Heartbeat internal
	Debug              bool               // enable Debug
	HandshakeValidator func([]byte) error // When you need to verify the custom data of the handshake request
	Serializer         serialize.Serializer
	GrpcOptions        = []grpc.DialOption{grpc.WithInsecure()}
)

func init() {
	Die = make(chan bool)
	Heartbeat = 30 * time.Second
	HandshakeValidator = func(_ []byte) error { return nil }
	Serializer = protobuf.NewSerializer()
}
