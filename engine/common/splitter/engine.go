package splitter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/rs/zerolog"
)

type Engine struct {
	enginesMu sync.RWMutex
	unit      *engine.Unit               // used to manage concurrency & shutdown
	log       zerolog.Logger             // used to log relevant actions with context
	engines   map[module.Engine]struct{} // stores registered engines
	channel   network.Channel            // the channel that this splitter listens on
}

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

func (e *Engine) RegisterEngine(engine module.Engine) error {
	e.enginesMu.Lock()
	defer e.enginesMu.Unlock()

	if _, ok := e.engines[engine]; ok {
		return errors.New("engine already registered with splitter")
	}

	e.engines[engine] = struct{}{}

	return nil
}

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
		err := e.ProcessLocal(event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(channel, originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		e.process(func(downstream module.Engine) error {
			return downstream.ProcessLocal(event)
		})

		return nil
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

		e.process(func(downstream module.Engine) error {
			return downstream.Process(channel, originID, event)
		})

		return nil
	})
}

// process calls the given function in parallel for all the engines that have
// registered with this splitter.
func (e *Engine) process(f func(module.Engine) error) {
	var wg sync.WaitGroup

	e.enginesMu.RLock()
	defer e.enginesMu.RUnlock()

	for eng := range e.engines {
		e.enginesMu.RUnlock()
		wg.Add(1)

		go func(downstream module.Engine, log zerolog.Logger) {
			defer wg.Done()

			err := f(downstream)

			if err != nil {
				engine.LogErrorWithMsg(log, "processing failed for downstream engine", err)
			}
		}(eng, e.log)

		e.enginesMu.RLock()
	}

	wg.Wait()
}
