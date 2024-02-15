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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// MessagingEngineConfig configures outbound message submission.
type MessagingEngineConfig struct {
	// RetryMax is the maximum number of times the engine will attempt to send
	// an outbound message before permanently giving up.
	RetryMax uint64
	// RetryBaseWait is the duration to wait between the two first send attempts.
	RetryBaseWait time.Duration
	// RetryJitterPercent is the percent jitter to add to each inter-retry wait.
	RetryJitterPercent uint64
}

// DefaultMessagingEngineConfig returns the config defaults. With 9 attempts and
// exponential backoff, this will retry for about 8m before giving up.
func DefaultMessagingEngineConfig() MessagingEngineConfig {
	return MessagingEngineConfig{
		RetryMax:           9,
		RetryBaseWait:      time.Second,
		RetryJitterPercent: 25,
	}
}

// MessagingEngine is an engine which sends and receives all DKG private messages.
// The same engine instance is used for the lifetime of a node and will be used
// for different DKG instances. The ReactorEngine is responsible for the lifecycle
// of components which are scoped one DKG instance, for example the DKGController.
// The dkg.BrokerTunnel handles routing messages to/from the current DKG instance.
type MessagingEngine struct {
	log     zerolog.Logger
	me      module.Local          // local object to identify the node
	conduit network.Conduit       // network conduit for sending and receiving private messages
	tunnel  *dkg.BrokerTunnel     // tunnel for relaying private messages to and from controllers
	config  MessagingEngineConfig // config for outbound message transmission

	messageHandler *engine.MessageHandler // encapsulates enqueueing messages from network
	notifier       engine.Notifier        // notifies inbound messages available for forwarding
	inbound        *fifoqueue.FifoQueue   // messages from the network, to be processed by DKG Controller

	component.Component
	cm *component.ComponentManager
}

var _ network.MessageProcessor = (*MessagingEngine)(nil)
var _ component.Component = (*MessagingEngine)(nil)

// NewMessagingEngine returns a new MessagingEngine.
func NewMessagingEngine(
	log zerolog.Logger,
	net network.EngineRegistry,
	me module.Local,
	tunnel *dkg.BrokerTunnel,
	collector module.MempoolMetrics,
	config MessagingEngineConfig,
) (*MessagingEngine, error) {
	log = log.With().Str("engine", "dkg_messaging").Logger()

	inbound, err := fifoqueue.NewFifoQueue(
		1000,
		fifoqueue.WithLengthMetricObserver(metrics.ResourceDKGMessage, collector.MempoolEntries))
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
		config:         config,
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
		return fmt.Errorf("unexpected failure to process inbound dkg message: %w", err)
	}
	return nil
}

// forwardInboundMessagesWorker reads queued inbound messages and forwards them
// through the broker tunnel to the DKG Controller for processing.
// This is a worker routine which runs for the lifetime of the engine.
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

// popNextInboundMessage pops one message from the queue and returns it as the
// appropriate type expected by the DKG controller.
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

// forwardInboundMessagesWhileAvailable retrieves all inbound messages from the queue and
// sends to the DKG Controller over the broker tunnel. Exists when the queue is empty.
func (e *MessagingEngine) forwardInboundMessagesWhileAvailable(ctx context.Context) {
	for {
		started := time.Now()
		message, ok := e.popNextInboundMessage()
		if !ok {
			return
		}

		select {
		case <-ctx.Done():
			return
		case e.tunnel.MsgChIn <- message:
			e.log.Debug().Dur("waited", time.Since(started)).Msg("forwarded DKG message to Broker")
			continue
		}
	}
}

// forwardOutboundMessagesWorker reads outbound DKG messages created by our DKG Controller
// and sends them to the appropriate other DKG participant. Each outbound message is sent
// async in an ad-hoc goroutine, which internally manages retry backoff for the message.
// This is a worker routine which runs for the lifetime of the engine.
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

// forwardOutboundMessage transmits message to the target DKG participant.
// Upon any error from the Unicast, we will retry with an exponential backoff.
// After a limited number of attempts, we will log an error and exit.
// The DKG protocol tolerates a number of failed private messages - these will
// be resolved by broadcasting complaints in later phases.
// Must be invoked as a goroutine.
func (e *MessagingEngine) forwardOutboundMessage(ctx context.Context, message msg.PrivDKGMessageOut) {
	backoff := retry.NewExponential(e.config.RetryBaseWait)
	backoff = retry.WithMaxRetries(e.config.RetryMax, backoff)
	backoff = retry.WithJitterPercent(e.config.RetryJitterPercent, backoff)

	started := time.Now()
	log := e.log.With().Str("target", message.DestID.String()).Logger()

	attempts := 0
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		err := e.conduit.Unicast(&message.DKGMessage, message.DestID)
		// TODO Unicast does not document expected errors, therefore we treat all errors as benign networking failures here
		if err != nil {
			log.Warn().
				Err(err).
				Int("attempt", attempts).
				Dur("send_time", time.Since(started)).
				Msgf("error sending dkg message on attempt %d - will retry...", attempts)
		}

		return retry.RetryableError(err)
	})

	// TODO Unicast does not document expected errors, therefore we treat all errors as benign networking failures here
	if err != nil {
		log.Error().
			Err(err).
			Int("total_attempts", attempts).
			Dur("total_send_time", time.Since(started)).
			Msgf("failed to send private dkg message after %d attempts - will not retry", attempts)
	}
}
