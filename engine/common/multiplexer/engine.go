package relay

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Engine struct {
	unit        *engine.Unit   // used to manage concurrency & shutdown
	log         zerolog.Logger // used to log relevant actions with context
	me          module.Local
	net         module.Network
	chanEngines map[network.Channel][]network.Engine // stores engine registration mapping
	conduits    map[network.Channel]network.Conduit  // stores conduits for all registered channels
	engines     map[network.Engine]struct{}          // set of all registered engines
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
) (*Engine, error) {
	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log.With().Str("engine", "multiplexer").Logger(),
		me:          me,
		net:         net,
		chanEngines: make(map[network.Channel][]network.Engine),
		conduits:    make(map[network.Channel]network.Conduit),
		engines:     make(map[network.Engine]struct{}),
	}

	return e, nil
}

// Register will subscribe the given engine with the multiplexer on the given channel, and all registered
// engines will be notified with incoming messages on the channel.
// The returned Conduit can be used to send messages to engines on other nodes subscribed to the same channel
func (e *Engine) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	_, ok := e.chanEngines[channel]

	if !ok {
		conduit, err := e.net.Register(channel, e)
		if err != nil {
			return nil, fmt.Errorf("failed to register multiplexer engine on channel %s: %w", channel, err)
		}

		e.conduits[channel] = conduit

		// initializes the engine set for the provided channel
		e.chanEngines[channel] = make([]network.Engine, 0)
	}

	for _, eng := range e.chanEngines[channel] {
		if eng == engine {
			return nil, fmt.Errorf("engine already registered on channel: %s", channel)
		}
	}

	e.chanEngines[channel] = append(e.chanEngines[channel], engine)
	e.engines[engine] = struct{}{}

	return e.conduits[channel], nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the multiplexer engine, this is true once all of the
// registered engines have started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready(func() {
		for engine := range e.engines {
			engine, ok := engine.(module.ReadyDoneAware)
			if ok {
				<-engine.Ready()
			}
		}

		// for _, engines := range e.chanEngines {
		// 	for _, engine := range engines {
		// 		engine, ok := engine.(module.ReadyDoneAware)
		// 		if ok {
		// 			<-engine.Ready()
		// 		}
		// 	}
		// }
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the multiplexer engine, this is true once all of the registered engines
// have stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		for engine := range e.engines {
			engine, ok := engine.(module.ReadyDoneAware)
			if ok {
				<-engine.Done()
			}
		}

		// for _, engines := range e.chanEngines {
		// 	for _, engine := range engines {
		// 		engine, ok := engine.(module.ReadyDoneAware)
		// 		if ok {
		// 			<-engine.Done()
		// 		}
		// 	}
		// }
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
	return fmt.Errorf("multiplexer engine does not process local events")
}

// Process processes the given event from the node with the given origin ID
// in a blocking manner. It returns the potential processing error when
// done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(channel, originID, event)
	})
}

// process fans out the given event in parallel to all the engines that have
// registered with this multiplexer on the given channel.
func (e *Engine) process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	engines, ok := e.chanEngines[channel]

	if !ok {
		log.Warn().Msgf("multiplexer has no engines registered on channel %s", channel)
		return nil

		// TODO: should we consider this an actual error?

		// return fmt.Errorf("multiplexer has no engines registered on channel %s", channel)
	}

	var wg sync.WaitGroup

	for _, eng := range engines {
		wg.Add(1)

		go func(e network.Engine, log zerolog.Logger) {
			defer wg.Done()

			err := e.Process(channel, originID, event)

			if err != nil {
				engine.LogErrorWithMsg(log, "downstream engine failed to process message", err)
			}
		}(eng, e.log)
	}

	wg.Wait()

	// TODO: should we just call Submit instead and return immediately?

	// for _, eng := range engines {
	// 	eng.Submit(channel, originID, event)
	// }

	return nil
}
