package engine

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

type Notifier interface {
	Notify()
}

type Message struct {
	OriginID flow.Identifier
	Payload  interface{}
}

type Store interface {
	Put(*Message) bool
	Get() (*Message, bool)
}

type Pattern struct {
	Match MatchFunc
	Store Store
}

type EnqueueFunc func(*Message)

type MatchFunc func(*Message) bool

type Enqueuer struct {
	log      zerolog.Logger
	notify   Notifier
	patterns []Pattern
}

func (e *Enqueuer) ProcessMessage(originID flow.Identifier, payload interface{}) {

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
			ok := pattern.Store.Put(msg)
			if !ok {
				log.Msg("failed to store message - discarding")
			}
			return
		}
	}

	log.Msg("discarding unknown message type")
}
