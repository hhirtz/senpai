package irc

import (
	"sync"
	"time"
)

type Typing struct {
	Target string
	Name   string
}

type Typings struct {
	l        sync.Mutex
	targets  map[Typing]time.Time
	timeouts chan Typing
	stops    chan Typing
}

func NewTypings() *Typings {
	ts := &Typings{
		targets:  map[Typing]time.Time{},
		timeouts: make(chan Typing, 16),
		stops:    make(chan Typing, 16),
	}
	go func() {
		for {
			t := <-ts.timeouts
			now := time.Now()
			ts.l.Lock()
			oldT, ok := ts.targets[t]
			if ok && 6.0 < now.Sub(oldT).Seconds() {
				delete(ts.targets, t)
				ts.l.Unlock()
				ts.stops <- t
			} else {
				ts.l.Unlock()
			}
		}
	}()
	return ts
}

func (ts *Typings) Stops() <-chan Typing {
	return ts.stops
}

func (ts *Typings) Active(target, name string) {
	t := Typing{target, name}
	ts.l.Lock()
	ts.targets[t] = time.Now()
	ts.l.Unlock()

	go func() {
		time.Sleep(6 * time.Second)
		ts.timeouts <- t
	}()
}

func (ts *Typings) Done(target, name string) {
	ts.l.Lock()
	delete(ts.targets, Typing{target, name})
	ts.l.Unlock()
}
