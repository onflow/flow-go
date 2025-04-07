package queue

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// KeyFunc defines a function type for computing the unique identifier for a message.
type KeyFunc func(msg *engine.Message) flow.Identifier

// Default key function.
var defaultKeyFunc = IdentifierOfMessage

type HeroStoreOption func(heroStore *HeroStore)

// WithMessageKeyFactory allows setting a custom function to generate the key for a message.
func WithMessageKeyFactory(f KeyFunc) HeroStoreOption {
	return func(h *HeroStore) {
		h.keyFactory = f
	}
}

// HeroStore is a FIFO (first-in-first-out) size-bound queue for maintaining engine.Message types.
// It is based on HeroQueue.
type HeroStore struct {
	q          *HeroQueue[*engine.Message]
	keyFactory KeyFunc
}

func NewHeroStore(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics, opts ...HeroStoreOption) *HeroStore {
	h := &HeroStore{
		q:          NewHeroQueue[*engine.Message](sizeLimit, logger, collector),
		keyFactory: defaultKeyFunc,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Put enqueues the message into the message store.
//
// Boolean returned variable determines whether enqueuing was successful, i.e.,
// put may be dropped if queue is full or already exists.
func (c *HeroStore) Put(message *engine.Message) bool {
	return c.q.Push(c.keyFactory(message), message)
}

// Get pops the queue, i.e., it returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., popping an empty queue returns false.
func (c *HeroStore) Get() (*engine.Message, bool) {
	return c.q.Pop()
}

// IdentifierOfMessage generates the unique identifier for a message.
func IdentifierOfMessage(msg *engine.Message) flow.Identifier {
	return flow.MakeID(msg)
}

// IdentifierOfMessageWithNonce generates an identifier with a nonce to prevent de-duplication.
func IdentifierOfMessageWithNonce(msg *engine.Message) flow.Identifier {
	return flow.MakeID(struct {
		*engine.Message
		Nonce uint64
	}{
		msg,
		uint64(time.Now().UnixNano()),
	})
}
