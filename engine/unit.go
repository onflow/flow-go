// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"sync"
)

type Unit struct {
	wg   sync.WaitGroup
	once sync.Once
	quit chan struct{}
}

func NewUnit() *Unit {
	return &Unit{
		quit: make(chan struct{}),
	}
}

func (u *Unit) Do(do func() error) error {
	select {
	case <-u.quit:
		return nil
	default:
	}
	u.wg.Add(1)
	defer u.wg.Done()
	return do()
}

func (u *Unit) Launch(launch func()) {
	select {
	case <-u.quit:
		return
	default:
	}
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		launch()
	}()
}

func (u *Unit) Ready(checks ...func()) <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		for _, check := range checks {
			check()
		}
		close(ready)
	}()
	return ready
}

func (u *Unit) Quit() <-chan struct{} {
	return u.quit
}

func (u *Unit) Done(actions ...func()) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		u.once.Do(func() {
			close(u.quit)
		})
		for _, action := range actions {
			action()
		}
		u.wg.Wait()
		close(done)
	}()
	return done
}
