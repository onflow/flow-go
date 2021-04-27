package engine

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

type Message struct {
	OriginID flow.Identifier
	Payload  interface{}
}

// MessageStore is the interface to abstract how messages are buffered in memory before
// being handled by the engine
type MessageStore interface {
	Put(*Message) bool
	Get() (*Message, bool)
}

type Pattern struct {
	// Match is a function to match a message to this pattern, typically by payload type.
	Match MatchFunc
	// Map is a function to apply to messages before storing them. if not provided, then the message won't get mapped.
	Map MapFunc
	// Store is an abstract message store where we will store the message upon receipt.
	Store MessageStore
	// BeforeStore is a hook for functions to be called when a message is stored.
	BeforeStore []OnMessageFunc
}

type OnMessageFunc func(*Message)

type MatchFunc func(*Message) bool

type MapFunc func(*Message) *Message

type MessageHandler struct {
	log      zerolog.Logger
	notify   chan struct{}
	patterns []Pattern
}

func NewMessageHandler(log zerolog.Logger, patterns ...Pattern) *MessageHandler {
	// the 1 message buffer is important to avoid the race condition.
	// the consumer might decide to listen to the notify channel, and drain the messages in the
	// message store, however there is a blind period start from the point the consumer learned
	// the message store is empty to the point the consumer start listening to the notifier channel
	// again. During this blind period, if the notifier had no buffer, then `doNotify` call will not
	// able to push message to the notifier channel, therefore has to drop the message and cause the
	// consumer waiting forever with unconsumed message in the message store.
	// having 1 message buffer covers the "blind period", so that during the blind priod if there is
	// a new message arrived, it will be buffered, and once the blind period is over, the consumer
	// will empty the buffer and start draining the message store again.
	notifier := make(chan struct{}, 1)
	enqueuer := &MessageHandler{
		log:      log.With().Str("component", "message_handler").Logger(),
		notify:   notifier,
		patterns: patterns,
	}
	return enqueuer
}

func (e *MessageHandler) Process(originID flow.Identifier, payload interface{}) (err error) {

	msg := &Message{
		OriginID: originID,
		Payload:  payload,
	}

	log := e.log.
		Warn().
		Str("msg_type", logging.Type(payload)).
		Hex("origin_id", originID[:])

	for _, pattern := range e.patterns {
		if pattern.Match(msg) {

			if pattern.Map != nil {
				msg = pattern.Map(msg)
			}

			for _, apply := range pattern.BeforeStore {
				apply(msg)
			}

			ok := pattern.Store.Put(msg)
			if !ok {
				log.Msg("failed to store message - discarding")
				return
			}

			e.doNotify()

			// message can only be matched by one pattern, and processed by one handler
			return
		}
	}

	log.Msg("discarding unknown message type")
	return
}

// notify the handler to pick new message from the queue
func (e *MessageHandler) doNotify() {
	select {
	// to prevent from getting blocked by dropping the notification if
	// there is no handler subscribing the channel.
	case e.notify <- struct{}{}:
	default:
	}
}

func (e *MessageHandler) GetNotifier() <-chan struct{} {
	return e.notify
}
