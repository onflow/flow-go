package dkg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// retryMax is the maximum number of times the engine will attempt to forward
// a message before permanently giving up.
const retryMax = 9

// retryBaseWait is the duration to wait between the two first tries.
// With 9 attempts and exponential backoff, this will retry for about
// 8m before giving up.
const retryBaseWait = 1 * time.Second

// retryJitterPct is the percent jitter to add to each inter-retry wait.
const retryJitterPct = 25

const nWorkers = 100

type MessagingEngineConfig struct {
	RetryMax           uint
	RetryBaseWait      time.Time
	RetryJitterPercent uint
}

//func DefaultMessagingEngineConfig

// MessagingEngine is an engine which sends and receives all DKG private messages.
// The same engine instance is used for the lifetime of a node and will be used
// for different DKG instances. The ReactorEngine is responsible for the lifecycle
// of components which are scoped one DKG instance, for example the DKGController.
// The dkg.BrokerTunnel handles routing messages to/from the current DKG instance.
type MessagingEngine struct {
	log     zerolog.Logger
	me      module.Local      // local object to identify the node
	conduit network.Conduit   // network conduit for sending and receiving private messages
	tunnel  *dkg.BrokerTunnel // tunnel for relaying private messages to and from controllers

	messageHandler *engine.MessageHandler // encapsulates enqueueing messages from network
	notifier       engine.Notifier        // notifies inbound messages available for forwarding
	inbound        *fifoqueue.FifoQueue   // messages from the network, to be processed by DKG Controller

	component.Component
	cm *component.ComponentManager
}

var _ network.MessageProcessor = (*MessagingEngine)(nil)
var _ component.Component = (*MessagingEngine)(nil)

// NewMessagingEngine returns a new engine.
func NewMessagingEngine(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	tunnel *dkg.BrokerTunnel,
) (*MessagingEngine, error) {
	log = log.With().Str("engine", "dkg.messaging").Logger()

	// TODO length observer metrics
	inbound, err := fifoqueue.NewFifoQueue(1000)
	if err != nil {
		return nil, fmt.Errorf("could not create inbound fifoqueue: %w", err)
	}

	notifier := engine.NewNotifier()
	messageHandler := engine.NewMessageHandler(log, notifier, engine.Pattern{
		Match: engine.MatchType[*msg.DKGMessage],
		Store: &engine.FifoMessageStore{FifoQueue: inbound},
	})

	eng := MessagingEngine{
		log:            log,
		me:             me,
		tunnel:         tunnel,
		messageHandler: messageHandler,
		notifier:       notifier,
		inbound:        inbound,
	}

	conduit, err := net.Register(channels.DKGCommittee, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register dkg network engine: %w", err)
	}
	eng.conduit = conduit

	eng.cm = component.NewComponentManagerBuilder().
		AddWorker(eng.forwardInboundMessagesWorker).
		AddWorker(eng.forwardOutboundMessagesWorker).
		Build()
	eng.Component = eng.cm

	return &eng, nil
}

// Process processes messages from the networking layer.
// No errors are expected during normal operation.
func (e *MessagingEngine) Process(channel channels.Channel, originID flow.Identifier, message any) error {
	err := e.messageHandler.Process(originID, message)
	if err != nil {
		if errors.Is(err, engine.IncompatibleInputTypeError) {
			e.log.Warn().Bool(logging.KeySuspicious, true).Msgf("%v delivered unsupported message %T through %v", originID, message, channel)
			return nil
		}
		// TODO add comment about Process errors...
		return fmt.Errorf("unexpected failure to process inbound dkg message")
	}
	return nil
}

func (e *MessagingEngine) forwardInboundMessagesWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()
	wake := e.notifier.Channel()
	for {
		select {
		case <-done:
			return
		case <-wake:
			e.forwardInboundMessagesWhileAvailable(ctx)
		}
	}
}

func (e *MessagingEngine) popNextInboundMessage() (msg.PrivDKGMessageIn, bool) {
	nextMessage, ok := e.inbound.Pop()
	if !ok {
		return msg.PrivDKGMessageIn{}, false
	}

	asEngineWrapper := nextMessage.(*engine.Message)
	asDKGMsg := asEngineWrapper.Payload.(*msg.DKGMessage)
	originID := asEngineWrapper.OriginID

	message := msg.PrivDKGMessageIn{
		DKGMessage: *asDKGMsg,
		OriginID:   originID,
	}
	return message, true
}

func (e *MessagingEngine) forwardInboundMessagesWhileAvailable(ctx context.Context) {
	for {
		message, ok := e.popNextInboundMessage()
		if !ok {
			return
		}

		select {
		case <-ctx.Done():
			return
		case e.tunnel.MsgChIn <- message:
			continue
		}
	}
}

func (e *MessagingEngine) forwardOutboundMessagesWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		case message := <-e.tunnel.MsgChOut:
			go e.forwardOutboundMessage(ctx, message)
		}
	}
}

// forwardOutboundMessage asynchronously attempts to forward a private
// DKG message to a single other DKG participant, on a best effort basis.
// Must be invoked as a goroutine.
func (e *MessagingEngine) forwardOutboundMessage(ctx context.Context, message msg.PrivDKGMessageOut) {
	backoff := retry.NewExponential(retryBaseWait)
	backoff = retry.WithMaxRetries(retryMax, backoff)
	backoff = retry.WithJitterPercent(retryJitterPct, backoff)

	log := e.log.With().Str("target", message.DestID.String()).Logger()

	attempts := 1
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		err := e.conduit.Unicast(&message.DKGMessage, message.DestID)
		// TODO Unicast fails to document expected errors, therefore we treat all errors as benign networking failures here
		if err != nil {
			log.Warn().
				Err(err).
				Int("attempt", attempts).
				Msgf("error sending dkg message on attempt %d - will retry...", attempts)
		}

		attempts++
		return retry.RetryableError(err)
	})

	// TODO Unicast fails to document expected errors, therefore we treat all errors as benign networking failures here
	if err != nil {
		log.Error().
			Err(err).
			Int("attempt", attempts).
			Msgf("failed to send private dkg message after %d attempts - will not retry", attempts)
	}
}
