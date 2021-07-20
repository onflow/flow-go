package splitter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// Engine is the splitter engine, which maintains a list of registered engines
// and passes every event it receives to each of these engines in parallel.
type Engine struct {
	enginesMu sync.RWMutex
	unit      *engine.Unit               // used to manage concurrency & shutdown
	log       zerolog.Logger             // used to log relevant actions with context
	engines   map[module.Engine]struct{} // stores registered engines
	channel   network.Channel            // the channel that this splitter listens on
}

// New creates a new splitter engine.
func New(
	log zerolog.Logger,
	channel network.Channel,
) *Engine {
	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log.With().Str("engine", "splitter").Logger(),
		engines: make(map[module.Engine]struct{}),
		channel: channel,
	}

	return e
}

// RegisterEngine registers a new engine with the splitter. Events
// that are received by the splitter after the engine has registered
// will be passed down to it.
func (e *Engine) RegisterEngine(engine module.Engine) error {
	e.enginesMu.Lock()
	defer e.enginesMu.Unlock()

	if _, ok := e.engines[engine]; ok {
		return errors.New("engine already registered with splitter")
	}

	e.engines[engine] = struct{}{}

	return nil
}

// UnregisterEngine unregisters an engine with the splitter. After
// the engine has been unregistered, the splitter will stop passing
// events to it. If the given engine was never registered, this is
// a noop.
func (e *Engine) UnregisterEngine(engine module.Engine) {
	e.enginesMu.Lock()
	defer e.enginesMu.Unlock()

	delete(e.engines, engine)
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the splitter engine, this is true once all of the
// registered engines have started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready(func() {
		e.enginesMu.RLock()
		defer e.enginesMu.RUnlock()
		for engine := range e.engines {
			e.enginesMu.RUnlock()
			<-engine.Ready()
			e.enginesMu.RLock()
		}
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the splitter engine, this is true once all of the registered engines
// have stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		e.enginesMu.RLock()
		defer e.enginesMu.RUnlock()
		for engine := range e.engines {
			e.enginesMu.RUnlock()
			<-engine.Done()
			e.enginesMu.RLock()
		}
	})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		e.enginesMu.RLock()
		defer e.enginesMu.RUnlock()

		for eng := range e.engines {
			eng.SubmitLocal(event)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		e.enginesMu.RLock()
		defer e.enginesMu.RUnlock()

		for eng := range e.engines {
			eng.Submit(channel, originID, event)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(func(downstream module.Engine) error {
			return downstream.ProcessLocal(event)
		})
	})
}

// Process processes the given event from the node with the given origin ID
// in a blocking manner. It returns the potential processing error when
// done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		if channel != e.channel {
			return fmt.Errorf("received event on unknown channel %s", channel)
		}

		return e.process(func(downstream module.Engine) error {
			return downstream.Process(channel, originID, event)
		})
	})
}

// process calls the given function in parallel for all the engines that have
// registered with this splitter.
func (e *Engine) process(processFunc func(module.Engine) error) error {
	e.enginesMu.RLock()

	numEngines := len(e.engines)
	errors := make(chan error, numEngines)

	for eng := range e.engines {
		eng := eng // https://golang.org/doc/faq#closures_and_goroutines

		go func() {
			errors <- processFunc(eng)
		}()
	}

	e.enginesMu.RUnlock()

	var multiErr *multierror.Error

	for i := 0; i < numEngines; i++ {
		if err := <-errors; err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	return multiErr.ErrorOrNil()
}
