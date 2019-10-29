// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Engine represents an engine to fake a consensus algorithm.
type Engine struct {
	log  zerolog.Logger
	prov Provider
	wg   *sync.WaitGroup
	stop chan struct{}
}

// New creates a new coldstuff engine.
func New(log zerolog.Logger, prov Provider) (*Engine, error) {
	e := &Engine{
		log:  log,
		prov: prov,
		wg:   &sync.WaitGroup{},
		stop: make(chan struct{}),
	}
	return e, nil
}

// Ready returns a channel that will close when the coldstuff engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when the coldstuff engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		close(e.stop)
		e.wg.Wait()
		close(done)
	}()
	return done
}

// Submit will submit an event for processing in a non-blocking manner.
func (e *Engine) Submit(event interface{}) {
	err := e.Process(event)
	if err != nil {
		e.log.Error().
			Err(err).
			Msg("could not process submitted event")
	}
}

// Identify will identify the given event with a unique ID.
func (e *Engine) Identify(event interface{}) ([]byte, error) {
	switch event.(type) {
	default:
		return nil, errors.Errorf("invalid event type (%T)", event)
	}
}

// Retrieve will retrieve an event by the given unique ID.
func (e *Engine) Retrieve() (interface{}, error) {
	return nil, errors.New("not implemented")
}

// Process will process an event submitted to the engine.
func (e *Engine) Process(event interface{}) error {
	var err error
	switch event.(type) {
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}
