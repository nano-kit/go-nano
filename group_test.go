package nano

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/nano-kit/go-nano/session"
)

func TestChannel_Add(t *testing.T) {
	c := NewGroup("test_add")

	var paraCount = 100
	w := make(chan bool, paraCount)
	for i := 0; i < paraCount; i++ {
		go func(id int) {
			s := session.New(nil)
			uid := strconv.FormatInt(int64(id+1), 10)
			s.Bind(uid)
			c.Add(s)
			w <- true
		}(i)
	}

	for i := 0; i < paraCount; i++ {
		<-w
	}

	if c.Count() != paraCount {
		t.Fatalf("count expect: %d, got: %d", paraCount, c.Count())
	}

	n := rand.Int63n(int64(paraCount)) + 1
	randUID := strconv.FormatInt(n, 10)
	if !c.Contains(randUID) {
		t.Fail()
	}

	// leave
	c.LeaveAll()
	if c.Count() != 0 {
		t.Fail()
	}
}
