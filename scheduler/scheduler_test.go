package scheduler

import (
	"testing"
	"time"
)

const (
	runCount = 500000
)

func TestRunAndRepeat(t *testing.T) {
	count := 0
	repeat := 0
	Repeat(func() {
		count++
		repeat++
	}, time.Millisecond)
	for i := 0; i < runCount; i++ {
		Run(func() { count++ })
	}
	time.Sleep(time.Millisecond) // wait all runs done
	if count != runCount+repeat {
		t.Error()
	}
}
