package splitter

import (
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
	return &Engine{
		unit:    engine.NewUnit(),
		log:     log.With().Str("engine", "splitter").Logger(),
		engines: make(map[module.Engine]struct{}),
		channel: channel,
	}
}

// RegisterEngine registers a new engine with the splitter. Events
// that are received by the splitter after the engine has registered
// will be passed down to it.
func (e *Engine) RegisterEngine(engine module.Engine) {
	e.enginesMu.Lock()
	defer e.enginesMu.Unlock()

	e.engines[engine] = struct{}{}
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
// started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		e.enginesMu.RLock()
		defer e.enginesMu.RUnlock()
		for eng := range e.engines {
			e.enginesMu.RUnlock()
			eng.SubmitLocal(event)
			e.enginesMu.RLock()
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		if channel != e.channel {
			e.log.Fatal().Err(fmt.Errorf("received event on unknown channel")).Str("channel", channel.String())
		}

		e.enginesMu.RLock()
		defer e.enginesMu.RUnlock()
		for eng := range e.engines {
			e.enginesMu.RUnlock()
			eng.Submit(channel, originID, event)
			e.enginesMu.RLock()
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
	count := 0
	errors := make(chan error)

	e.enginesMu.RLock()
	for eng := range e.engines {
		e.enginesMu.RUnlock()

		count += 1
		go func(downstream module.Engine) {
			errors <- processFunc(downstream)
		}(eng)

		e.enginesMu.RLock()
	}
	e.enginesMu.RUnlock()

	var multiErr *multierror.Error

	for i := 0; i < count; i++ {
		multiErr = multierror.Append(multiErr, <-errors)
	}

	return multiErr.ErrorOrNil()
}
