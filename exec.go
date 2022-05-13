package net

import (
	"container/list"
	"sync"
	"time"
)

type IExecutor interface {
	Exec(func())
}

type executor struct {
	q          chan func()
	ready      bool
	closed     bool
	closedChan chan any
	lock       sync.RWMutex
	tickers    *list.List
}

func NewExecutor() *executor {
	return NewExecutorWithSize(16)
}

func NewExecutorWithSize(size int) *executor {
	return &executor{
		q:       make(chan func(), size),
		tickers: list.New(),
	}
}

func (e *executor) Exec(f func()) {
	if !e.ready {
		e.lock.Lock()
		go func() {
			for {
				select {
				case f := <-e.q:
					f()
				case <-e.closedChan:
					e.lock.Lock()
					e.closed = true
					e.lock.Unlock()
				}
			}

		}()
		e.ready = true
		e.lock.Unlock()
	}

	e.lock.RLock()
	defer e.lock.RUnlock()
	if !e.closed {
		e.q <- f
	}
}

func (e *executor) After(d time.Duration, f func()) *time.Timer {
	return time.AfterFunc(d, func() { e.Exec(f) })
}

func (e *executor) Ticker(d time.Duration, f func(*time.Ticker, *time.Time)) IStopper {
	ticker := time.NewTicker(d)
	stop := make(chan any)
	stopper := NewStopper(stop)
	go func(element *list.Element) {
		for {
			select {
			case t := <-ticker.C:
				e.Exec(func() { f(ticker, &t) })
			case <-stop:
				e.Exec(func() {
					e.tickers.Remove(element)
				})
				return
			}
		}
	}(e.tickers.PushBack(stopper))
	return stopper
}

func (e *executor) Shutdown() {
	e.Exec(func() {
		e.lock.Lock()
		defer e.lock.Unlock()
		if e.closed {
			return
		}
		for e := e.tickers.Front(); e != nil; e = e.Next() {
			e.Value.(IStopper).Stop()
		}
		e.tickers = list.New()
		e.closed = true
	})
}

type IStopper interface {
	Stop()
}

type ChanStopper struct {
	stopped bool
	// lock    sync.Mutex
	stop chan<- any
}

func NewStopper(c chan<- any) IStopper {
	return &ChanStopper{stop: c}
}

func (s *ChanStopper) Stop() {
	if s.stopped {
		return
	}
	//s.lock.Lock()
	//defer s.lock.Unlock()
	s.stopped = true
	s.stop <- 1
	close(s.stop)
}
