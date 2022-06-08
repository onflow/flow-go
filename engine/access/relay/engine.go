package relay

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// Relay engine relays all the messages that are received to the given network for the corresponding channel
type Engine struct {
	unit     *engine.Unit                        // used to manage concurrency & shutdown
	log      zerolog.Logger                      // used to log relevant actions with context
	conduits map[network.Channel]network.Conduit // conduits for unstaked network
}

func New(
	log zerolog.Logger,
	channels network.ChannelList,
	net network.Network,
	unstakedNet network.Network,
) (*Engine, error) {
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "relay").Logger(),
		conduits: make(map[network.Channel]network.Conduit),
	}

	for _, channel := range channels {
		_, err := net.Register(channel, e)
		if err != nil {
			return nil, fmt.Errorf("could not register relay engine on channel: %w", err)
		}

		conduit, err := unstakedNet.Register(channel, e)
		if err != nil {
			return nil, fmt.Errorf("could not register relay engine on unstaked network channel: %w", err)
		}
		e.conduits[channel] = conduit
	}

	return e, nil
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
		err := e.ProcessLocal(event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return fmt.Errorf("relay engine does not process local events")
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

// Process processes the given event from the node with the given origin ID
// in a blocking manner. It returns the potential processing error when
// done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(channel, originID, event)
	})
}

func (e *Engine) process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	conduit, ok := e.conduits[channel]

	if !ok {
		return fmt.Errorf("received message on unknown channel %s", channel)
	}

	e.log.Trace().Interface("event", event).Str("channel", channel.String()).Str("originID", originID.String()).Msg("relaying message")

	// We use a dummy target ID here so that events are broadcast to the entire network
	if err := conduit.Publish(event, flow.ZeroID); err != nil {
		return fmt.Errorf("could not relay message: %w", err)
	}

	return nil
}
