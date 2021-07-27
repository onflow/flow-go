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
	// Map is a function to apply to messages before storing them. If not provided, then the message is stored in its original form.
	Map MapFunc
	// Store is an abstract message store where we will store the message upon receipt.
	Store MessageStore
}

type FilterFunc func(*Message) bool

type MatchFunc func(*Message) bool

type MapFunc func(*Message) (*Message, bool)

type MessageHandler struct {
	log      zerolog.Logger
	notifier Notifier
	patterns []Pattern
}

func NewMessageHandler(log zerolog.Logger, notifier Notifier, patterns ...Pattern) *MessageHandler {
	return &MessageHandler{
		log:      log.With().Str("component", "message_handler").Logger(),
		notifier: notifier,
		patterns: patterns,
	}
}

// Process iterates over the internal processing patterns and determines if the payload matches.
// The _first_ matching pattern processes the payload.
// Returns
//  * IncompatibleInputTypeError if no matching processor was found
//  * All other errors are potential symptoms of internal state corruption or bugs (fatal).
func (e *MessageHandler) Process(originID flow.Identifier, payload interface{}) error {
	msg := &Message{
		OriginID: originID,
		Payload:  payload,
	}

	for _, pattern := range e.patterns {
		if pattern.Match(msg) {
			var keep bool
			if pattern.Map != nil {
				msg, keep = pattern.Map(msg)
				if !keep {
					return nil
				}
			}

			ok := pattern.Store.Put(msg)
			if !ok {
				e.log.Warn().
					Str("msg_type", logging.Type(payload)).
					Hex("origin_id", originID[:]).
					Msg("failed to store message - discarding")
				return nil
			}
			e.notifier.Notify()

			// message can only be matched by one pattern, and processed by one handler
			return nil
		}
	}

	return fmt.Errorf("no matching processor for message of type %T from origin %x: %w", payload, originID[:], IncompatibleInputTypeError)
}

func (e *MessageHandler) GetNotifier() <-chan struct{} {
	return e.notifier.Channel()
}
