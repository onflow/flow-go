package runner

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/engine"
)

type Compliance struct {
	unit     *engine.Unit // used to control startup/shutdown
	hotstuff *Hotstuff
	sync     *Synchronization
}

func (e *Compliance) Ready() <-chan struct{} {
	if e.sync == nil {
		panic("must initialize compliance engine with synchronization module")
	}
	if e.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	return e.unit.Ready(func() {
		<-e.sync.Ready()
		<-e.hotstuff.Ready()
	})
}

func (e *Compliance) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.sync.Done()
		<-e.hotstuff.Done()
	})
}

type Hotstuff struct {
	runner SingleRunner // lock for preventing concurrent state transitions
}

func (el *Hotstuff) Ready() <-chan struct{} {
	return el.runner.Start(el.loop)
}

// Done implements interface module.ReadyDoneAware
func (el *Hotstuff) Done() <-chan struct{} {
	return el.runner.Abort()
}

// Wait implements a function to wait for the event loop to exit.
func (el *Hotstuff) Wait() <-chan struct{} {
	return el.runner.Completed()
}

func (l *Hotstuff) loop() {
	for {
		shutdownSignal := l.runner.ShutdownSignal()
		select {
		case <-shutdownSignal:
			return
		}
	}
}

type Synchronization struct {
	unit *engine.Unit
}

func (e *Synchronization) Ready() <-chan struct{} {
	e.unit.Launch(e.checkLoop)
	return e.unit.Ready()
}

func (e *Synchronization) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Synchronization) checkLoop() {
CheckLoop:
	for {
		select {
		case <-e.unit.Quit():
			break CheckLoop
		}
	}
}

func start(hs []*Hotstuff, ss []*Synchronization, cs []*Compliance) {
	fmt.Printf("started runners\n")
	n := len(hs)
	// start runner
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			<-cs[i].Ready()
			<-hs[i].Wait()
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func stop(hs []*Hotstuff, ss []*Synchronization, cs []*Compliance) {
	// stop runner
	n := len(hs)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			<-cs[i].Done()
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestConcurrency(t *testing.T) {
	n := 3
	hs := make([]*Hotstuff, 0, n)
	ss := make([]*Synchronization, 0, n)
	cs := make([]*Compliance, 0, n)

	// create runner
	for i := 0; i < n; i++ {
		h := &Hotstuff{
			runner: NewSingleRunner(),
		}
		s := &Synchronization{
			unit: engine.NewUnit(),
		}
		c := &Compliance{
			unit:     engine.NewUnit(),
			sync:     s,
			hotstuff: h,
		}

		hs = append(hs, h)
		ss = append(ss, s)
		cs = append(cs, c)
	}

	fmt.Printf("created runners\n")
	go start(hs, ss, cs)

	time.Sleep(1 * time.Second)

	fmt.Printf("stopping runners\n")
	stop(hs, ss, cs)
	fmt.Printf("stopped runners\n")
}
