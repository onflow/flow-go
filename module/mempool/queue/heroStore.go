package queue

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
)

var defaultMsgEntityFactoryFunc = NewMessageEntity

type HeroStoreOption func(heroStore *HeroStore)

func WithMessageEntityFactory(f func(message *engine.Message) MessageEntity) HeroStoreOption {
	return func(heroStore *HeroStore) {
		heroStore.msgEntityFactory = f
	}
}

// HeroStore is a FIFO (first-in-first-out) size-bound queue for maintaining engine.Message types.
// It is based on HeroQueue.
type HeroStore struct {
	q                *HeroQueue
	msgEntityFactory func(message *engine.Message) MessageEntity
}

func NewHeroStore(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics, opts ...HeroStoreOption) *HeroStore {
	h := &HeroStore{
		q:                NewHeroQueue(sizeLimit, logger, collector),
		msgEntityFactory: defaultMsgEntityFactoryFunc,
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
	return c.q.Push(c.msgEntityFactory(message))
}

// Get pops the queue, i.e., it returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., popping an empty queue returns false.
func (c *HeroStore) Get() (*engine.Message, bool) {
	head, ok := c.q.Pop()
	if !ok {
		return nil, false
	}

	msg := head.(MessageEntity).Msg
	return &msg, true
}
