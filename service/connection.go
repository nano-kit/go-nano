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

package service

import (
	"strconv"
	"sync/atomic"

	"github.com/nano-kit/go-nano/internal/env"
)

const (
	// reserve high 32 bits for gate id
	sessionIDMask = 0xffffffff
	gateIDShift   = 32
)

// Connections is a global variable which is used by session.
var Connections = newConnectionService()

type SID int64

func (s SID) String() string {
	gate := int64(s >> gateIDShift)
	session := int64(s & sessionIDMask)
	str := strconv.FormatInt(session, 10)
	if gate == 0 {
		return str
	}
	return strconv.FormatInt(gate, 10) + "_" + str
}

type connectionService struct {
	sid uint32
}

func newConnectionService() *connectionService {
	return &connectionService{sid: 0}
}

// SessionID returns the session id
func (c *connectionService) SessionID() SID {
	s := SID(atomic.AddUint32(&c.sid, 1))
	g := SID(env.GateID) << gateIDShift
	return g | s
}
