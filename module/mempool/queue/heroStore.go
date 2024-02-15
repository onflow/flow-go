package queue

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/queue/internal"
)

type HeroStoreConfig struct {
	SizeLimit uint32
	Collector module.HeroCacheMetrics
}

type HeroStoreConfigOption func(builder *HeroStoreConfig)

func WithHeroStoreSizeLimit(sizeLimit uint32) HeroStoreConfigOption {
	return func(builder *HeroStoreConfig) {
		builder.SizeLimit = sizeLimit
	}
}

func WithHeroStoreCollector(collector module.HeroCacheMetrics) HeroStoreConfigOption {
	return func(builder *HeroStoreConfig) {
		builder.Collector = collector
	}
}

// HeroStore is a FIFO (first-in-first-out) size-bound queue for maintaining engine.Message types.
// It is based on HeroQueue.
type HeroStore struct {
	q *HeroQueue
}

func NewHeroStore(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics,
) *HeroStore {
	return &HeroStore{
		q: NewHeroQueue(sizeLimit, logger, collector),
	}
}

// Put enqueues the message into the message store.
//
// Boolean returned variable determines whether enqueuing was successful, i.e.,
// put may be dropped if queue is full or already exists.
func (c *HeroStore) Put(message *engine.Message) bool {
	return c.q.Push(internal.NewMessageEntity(message))
}

// Get pops the queue, i.e., it returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., popping an empty queue returns false.
func (c *HeroStore) Get() (*engine.Message, bool) {
	head, ok := c.q.Pop()
	if !ok {
		return nil, false
	}

	msg := head.(internal.MessageEntity).Msg
	return &msg, true
}
