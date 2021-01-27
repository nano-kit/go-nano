package service

import (
	"fmt"
	"github.com/nano-kit/go-nano/internal/env"
	"testing"
)

func TestSID_String(t *testing.T) {
	table := []struct {
		in  SID
		out string
	}{
		{100, "100"},
		{5999876, "5999876"},
		{4294967295, "4294967295"},
		{281474976710655, "65535_4294967295"},
		{1<<gateIDShift | 100, "1_100"},
		{10<<gateIDShift | 100, "10_100"},
	}
	for _, tt := range table {
		tt := tt
		t.Run(fmt.Sprint(tt.in), func(t *testing.T) {
			s := tt.in.String()
			if s != tt.out {
				t.Errorf("got %q, want %q", s, tt.out)
			}
		})
	}
}

func TestConnectionService_SessionID(t *testing.T) {
	s := newConnectionService()
	sid := s.SessionID()
	if sid != 1 {
		t.Errorf("got %q, want %q", sid, 1)
	}
	sid = s.SessionID()
	if sid != 2 {
		t.Errorf("got %q, want %q", sid, 2)
	}
	s.sid = 0xffffffff
	sid = s.SessionID()
	if sid != 0 {
		t.Errorf("got %q, want %q", sid, 0)
	}
	env.GateID = 1
	sid = s.SessionID()
	env.GateID = 0
	if sid != 0x100000001 {
		t.Errorf("got %q, want %q", sid, "1_1")
	}
}
