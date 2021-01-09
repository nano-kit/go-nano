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

package scheduler

import (
	"runtime/debug"
	"time"

	"github.com/aclisp/go-nano/internal/log"
)

// LocalScheduler schedules task to a customized goroutine
type LocalScheduler interface {
	Schedule(Task)
}

// Task is a function
type Task func()

// SystemTimedSched is the library level timed-scheduler
var systemTimedSched *TimedSched = NewTimedSched(1)

func try(f Task) Task {
	return func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("handle task panic: %+v\n%s", err, debug.Stack())
			}
		}()
		f()
	}
}

// Close stops the scheduler
func Close() {
	systemTimedSched.Close()
	systemTimedSched = nil
	log.Print("scheduler stopped")
}

// Run add task to scheduler for immediate execution
func Run(task Task) {
	systemTimedSched.Run(try(task))
}

type repeatableTask struct {
	Task
	interval time.Duration
}

func (r repeatableTask) run() {
	now := time.Now()
	r.Task()
	systemTimedSched.Put(r.run, now.Add(r.interval))
}

// Repeat runs the task repeatly at every interval
func Repeat(task Task, interval time.Duration) {
	r := repeatableTask{try(task), interval}
	now := time.Now()
	systemTimedSched.Put(r.run, now.Add(interval))
}
