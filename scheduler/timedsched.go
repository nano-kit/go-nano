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
	"container/heap"
	"sync"
	"time"
)

type runnableFunc func()

type timedFunc struct {
	execute func()
	ts      time.Time
}

// a heap for sorted timed function
type timedFuncHeap []timedFunc

func (h timedFuncHeap) Len() int            { return len(h) }
func (h timedFuncHeap) Less(i, j int) bool  { return h[i].ts.Before(h[j].ts) }
func (h timedFuncHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *timedFuncHeap) Push(x interface{}) { *h = append(*h, x.(timedFunc)) }
func (h *timedFuncHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1].execute = nil // avoid memory leak
	*h = old[0 : n-1]
	return x
}

// TimedSched represents the control struct for timed parallel scheduler.
// This scheduler also executes runnable tasks at the same go routine.
// It is inefficient for programs create and delete timers frequently, because
// leaving a lot of timers in the heap slows down addtimer.
// If your program create and delete a lot of short period timers, use a
// time-wheel mechanism such as https://github.com/andy2046/tik
type TimedSched struct {
	// prepending tasks
	prependTasks    []timedFunc
	prependLock     sync.Mutex
	chPrependNotify chan struct{}

	// tasks will be distributed through chTask
	chTask     chan timedFunc
	chRunnable chan runnableFunc

	dieOnce sync.Once
	die     chan struct{}
	exit    chan struct{}
}

// NewTimedSched creates a parallel-scheduler with given parallelization
func NewTimedSched(parallel int) *TimedSched {
	ts := new(TimedSched)
	ts.chTask = make(chan timedFunc)
	ts.chRunnable = make(chan runnableFunc, 1<<8)
	ts.die = make(chan struct{})
	ts.exit = make(chan struct{}, parallel+1) // parallel+1 pending go routines
	ts.chPrependNotify = make(chan struct{}, 1)

	for i := 0; i < parallel; i++ {
		go ts.sched()
	}
	go ts.prepend()
	return ts
}

func (ts *TimedSched) sched() {
	var tasks timedFuncHeap
	timer := time.NewTimer(0)
	drained := false
	defer func() {
		timer.Stop()
		ts.exit <- struct{}{}
	}()
	for {
		select {
		case runnable := <-ts.chRunnable:
			runnable()
		case task := <-ts.chTask:
			now := time.Now()
			if now.After(task.ts) {
				// already delayed! execute immediately
				task.execute()
			} else {
				heap.Push(&tasks, task)
				// properly reset timer to trigger based on the top element
				stopped := timer.Stop()
				if !stopped && !drained {
					<-timer.C
				}
				timer.Reset(tasks[0].ts.Sub(now))
				drained = false
			}
		case now := <-timer.C:
			drained = true
			for tasks.Len() > 0 {
				if now.After(tasks[0].ts) {
					heap.Pop(&tasks).(timedFunc).execute()
				} else {
					timer.Reset(tasks[0].ts.Sub(now))
					drained = false
					break
				}
			}
		case <-ts.die:
			return
		}
	}
}

func (ts *TimedSched) prepend() {
	var tasks []timedFunc
	defer func() {
		ts.exit <- struct{}{}
	}()
	for {
		select {
		case <-ts.chPrependNotify:
			ts.prependLock.Lock()
			// keep cap to reuse slice
			if cap(tasks) < cap(ts.prependTasks) {
				tasks = make([]timedFunc, 0, cap(ts.prependTasks))
			}
			tasks = tasks[:len(ts.prependTasks)]
			copy(tasks, ts.prependTasks)
			for k := range ts.prependTasks {
				ts.prependTasks[k].execute = nil // avoid memory leak
			}
			ts.prependTasks = ts.prependTasks[:0]
			ts.prependLock.Unlock()

			for k := range tasks {
				select {
				case ts.chTask <- tasks[k]:
					tasks[k].execute = nil // avoid memory leak
				case <-ts.die:
					return
				}
			}
			tasks = tasks[:0]
		case <-ts.die:
			return
		}
	}
}

// Put a function 'f' awaiting to be executed at 'deadline'
func (ts *TimedSched) Put(f func(), deadline time.Time) {
	ts.prependLock.Lock()
	ts.prependTasks = append(ts.prependTasks, timedFunc{f, deadline})
	ts.prependLock.Unlock()

	select {
	case ts.chPrependNotify <- struct{}{}:
	default:
	}
}

// Run a function 'f' immediately
func (ts *TimedSched) Run(f func()) {
	ts.chRunnable <- f
}

// Close terminates this scheduler
func (ts *TimedSched) Close() {
	ts.dieOnce.Do(func() {
		close(ts.die)
		for i := 0; i < cap(ts.exit); i++ {
			<-ts.exit
		}
	})
}
