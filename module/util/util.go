package util

import (
	"sync"

	"github.com/onflow/flow-go/module"
)

// DefaultFallbackStrategy simple fallback strategy assuming clients are stored in a list
// this strategy will manage the active client index
type DefaultFallbackStrategy struct {
	numOfClients int
	clientIndex  int
}

// ClientIndex returns current client index
func (strat *DefaultFallbackStrategy) ClientIndex() int {
	return strat.clientIndex
}

// Failure increments client index, when max is reached clientIndex is reset to 0
func (strat *DefaultFallbackStrategy) Failure() {
	if strat.clientIndex == strat.numOfClients {
		strat.clientIndex = 0
		return
	}

	strat.clientIndex++
}

// NewDefaultFallbackStrategy returns DefaultFallbackStrategy with client index initialized to 0
func NewDefaultFallbackStrategy(numOfClients int) module.FallbackStrategy {
	return &DefaultFallbackStrategy{numOfClients: numOfClients, clientIndex: 0}
}

// AllReady calls Ready on all input components and returns a channel that is
// closed when all input components are ready.
func AllReady(components ...module.ReadyDoneAware) <-chan struct{} {
	ready := make(chan struct{})
	var wg sync.WaitGroup

	for _, component := range components {
		wg.Add(1)
		go func(c module.ReadyDoneAware) {
			<-c.Ready()
			wg.Done()
		}(component)
	}

	go func() {
		wg.Wait()
		close(ready)
	}()

	return ready
}

// AllDone calls Done on all input components and returns a channel that is
// closed when all input components are done.
func AllDone(components ...module.ReadyDoneAware) <-chan struct{} {
	done := make(chan struct{})
	var wg sync.WaitGroup

	for _, component := range components {
		wg.Add(1)
		go func(c module.ReadyDoneAware) {
			<-c.Done()
			wg.Done()
		}(component)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	return done
}
