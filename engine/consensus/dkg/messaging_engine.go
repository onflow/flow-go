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
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			e.log.Fatal().Err(err).Str("origin", e.me.NodeID().String()).Msg("failed to submit local message")
		}
	})
}

// Submit implements the network Engine interface
func (e *MessagingEngine) Submit(_ network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if engine.IsInvalidInputError(err) {
			e.log.Error().Err(err).Str("origin", originID.String()).Msg("failed to submit dropping invalid input message")
		} else if err != nil {
			e.log.Fatal().Err(err).Str("origin", originID.String()).Msg("failed to submit message unknown error")
		}
	})
}

// ProcessLocal implements the network Engine interface
func (e *MessagingEngine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			e.log.Fatal().Err(err).Str("origin", e.me.NodeID().String()).Msg("failed to process local message")
		}

		return nil
	})
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
		return engine.NewInvalidInputErrorf("expecting input with type msg.DKGMessage, but got %T", event)
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
	e.unit.Launch(func() {
		expRetry, err := retry.NewExponential(retryMilliseconds)
		if err != nil {
			e.log.Fatal().Err(err).Msg("failed to create retry mechanism")
		}

		maxedExpRetry := retry.WithMaxRetries(retryMax, expRetry)
		attempts := 1
		err = retry.Do(e.unit.Ctx(), maxedExpRetry, func(ctx context.Context) error {
			err := e.conduit.Unicast(&message.DKGMessage, message.DestID)
			if err != nil {
				e.log.Warn().Err(err).Msgf("error sending dkg message retrying (%x)", attempts)
			}

			attempts++
			return retry.RetryableError(err)
		})

		// Various network can conditions can result in errors while forwarding outbound messages,
		// because failure to send an individual DKG message doesn't necessarily result in local or global DKG failure
		// it is acceptable to log the error and move on.
		if err != nil {
			e.log.Error().Err(err).Msg("error sending dkg message")
		}
	})
}
