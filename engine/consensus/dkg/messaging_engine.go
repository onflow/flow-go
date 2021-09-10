package dkg

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/network"
)

// retryMax is the maximum number of times the engine will attempt to forward
// a message
const retryMax = 5

// retryMilliseconds is the number of milliseconds to wait between the two first tries
const retryMilliseconds = 1000 * time.Millisecond

// MessagingEngine is a network engine that enables DKG nodes to exchange
// private messages over the network.
type MessagingEngine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	me      module.Local      // local object to identify the node
	conduit network.Conduit   // network conduit for sending and receiving private messages
	tunnel  *dkg.BrokerTunnel // tunnel for relaying private messages to and from controllers
}

// NewMessagingEngine returns a new engine.
func NewMessagingEngine(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	tunnel *dkg.BrokerTunnel) (*MessagingEngine, error) {

	log := logger.With().Str("engine", "dkg-processor").Logger()

	eng := MessagingEngine{
		unit:   engine.NewUnit(),
		log:    log,
		me:     me,
		tunnel: tunnel,
	}

	var err error
	eng.conduit, err = net.Register(engine.DKGCommittee, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register dkg network engine: %w", err)
	}

	eng.unit.Launch(eng.forwardOutgoingMessages)

	return &eng, nil
}

// Ready implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully
// started.
func (e *MessagingEngine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully stopped.
func (e *MessagingEngine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal implements the network Engine interface
func (e *MessagingEngine) SubmitLocal(event interface{}) {
	e.Submit(engine.DKGCommittee, e.me.NodeID(), event)
}

// Submit implements the network Engine interface
func (e *MessagingEngine) Submit(_ network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(engine.DKGCommittee, originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal implements the network Engine interface
func (e *MessagingEngine) ProcessLocal(event interface{}) error {
	return e.Process(engine.DKGCommittee, e.me.NodeID(), event)
}

// Process implements the network Engine interface
func (e *MessagingEngine) Process(_ network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *MessagingEngine) process(originID flow.Identifier, event interface{}) error {
	switch v := event.(type) {
	case *msg.DKGMessage:
		// messages are forwarded async rather than sync, because otherwise the message queue
		// might get full when it's slow to process DKG messages synchronously and impact 
		// block rate.
		e.forwardInboundMessageAsync(originID, v)
		return nil
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *MessagingEngine) forwardInboundMessageAsync(originID flow.Identifier, message *msg.DKGMessage) {
	e.unit.Launch(func() {
		e.tunnel.SendIn(
			msg.PrivDKGMessageIn{
				DKGMessage: *message,
				OriginID:   originID,
			},
		)
	})
}

func (e *MessagingEngine) forwardOutgoingMessages() {
	for {
		select {
		case msg := <-e.tunnel.MsgChOut:
			e.forwardOutboundMessageAsync(msg)
		case <-e.unit.Quit():
			return
		}
	}
}

func (e *MessagingEngine) forwardOutboundMessageAsync(message msg.PrivDKGMessageOut) {
	f := func() {
		expRetry, err := retry.NewExponential(retryMilliseconds)
		if err != nil {
			e.log.Fatal().Err(err).Msg("failed to create retry mechanism")
		}

		maxedExpRetry := retry.WithMaxRetries(retryMax, expRetry)
		err = retry.Do(e.unit.Ctx(), maxedExpRetry, func(ctx context.Context) error {
			err := e.conduit.Unicast(&message.DKGMessage, message.DestID)
			if err != nil {
				e.log.Warn().Err(err).Msg("error sending dkg message retrying")
			}
			return retry.RetryableError(err)
		})
		if err != nil {
			e.log.Error().Err(err).Msg("error sending dkg message")
		}
	}

	e.unit.Launch(f)
}
