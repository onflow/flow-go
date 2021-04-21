package engine

import (
	"fmt"

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
	// OnStore is a hook for functions to be called when a message is stored.
	OnStore []OnMessageFunc
}

type OnMessageFunc func(*Message)

type MatchFunc func(*Message) bool

type MapFunc func(*Message) *Message

type MessageHandler struct {
	log      zerolog.Logger
	notify   chan<- struct{}
	patterns []Pattern
}

func NewMessageHandler(log zerolog.Logger, patterns ...Pattern) (*MessageHandler, <-chan struct{}) {
	notifier := make(chan struct{})
	enqueuer := &MessageHandler{
		log:      log.With().Str("component", "message_handler").Logger(),
		notify:   notifier,
		patterns: patterns,
	}
	return enqueuer, notifier
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

			for _, apply := range pattern.OnStore {
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

// no-op implementation of network.Engine
// TODO: replace with single-method MessageProcessor interface
func (e *MessageHandler) Submit(_ flow.Identifier, _ interface{}) { panic("not implemented") }
func (e *MessageHandler) SubmitLocal(_ interface{})               { panic("not implemented") }
func (e *MessageHandler) ProcessLocal(_ interface{}) error        { panic("not implemented") }

// notify the handler to pick new message from the queue
func (e *MessageHandler) doNotify() {
	fmt.Println("notify")
	select {
	// to prevent from getting blocked by dropping the notification if
	// there is no handler subscribing the channel.
	case e.notify <- struct{}{}:
	default:
	}
}
