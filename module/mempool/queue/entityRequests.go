package queue

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/queue/internal"
)

// EntityRequestStore is a FIFO (first-in-first-out) size-bound queue for maintaining EntityRequests.
// It is designed to be utilized at the common ProviderEngine to maintain and respond entity requests.
type EntityRequestStore struct {
	mu        sync.RWMutex
	cache     *herocache.Cache
	sizeLimit uint
}

var _ mempool.EntityRequestStore = (*EntityRequestStore)(nil)

func NewEntityRequestStore(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *EntityRequestStore {
	return &EntityRequestStore{
		cache: herocache.NewCache(
			sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.NoEjection,
			logger.With().Str("mempool", "entity-request-store").Logger(),
			collector),
		sizeLimit: uint(sizeLimit),
	}
}

// Put enqueues the entity request message into the message store.
//
// Boolean returned variable determines whether enqueuing was successful, i.e.,
// put may be dropped if queue is full or already exists.
func (e *EntityRequestStore) Put(message *engine.Message) bool {
	request := message.Payload.(*messages.EntityRequest)
	return e.push(message.OriginID, request.EntityIDs)
}

// Get pops the queue, i.e., it returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., poping an empty queue returns false.
func (e *EntityRequestStore) Get() (*engine.Message, bool) {
	originId, request, ok := e.pop()
	if !ok {
		return nil, false
	}

	return &engine.Message{
		OriginID: originId,
		Payload:  request,
	}, true
}

func (e *EntityRequestStore) Size() uint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.cache.Size()
}

// push stores request into the queue.
// Boolean returned variable determines whether push was successful, i.e.,
// push may be dropped if queue is full or already exists.
func (e *EntityRequestStore) push(originId flow.Identifier, entityIds []flow.Identifier) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cache.Size() >= e.sizeLimit {
		// we check size before attempt on a push,
		// although HeroCache is on no-ejection mode and discards pushes beyond limit,
		// we save an id computation by just checking the size here.
		return false
	}

	req := internal.NewRequestEntity(originId, entityIds)
	return e.cache.Add(req.ID(), req)
}

// pop removes and returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., poping an empty queue returns false.
func (e *EntityRequestStore) pop() (flow.Identifier, []flow.Identifier, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	head, ok := e.cache.Head()
	if !ok {
		// cache is empty, and there is no head yet to pop.
		return flow.Identifier{}, nil, false
	}

	e.cache.Remove(head.ID())
	request := head.(internal.RequestEntity)
	return request.OriginId, request.EntityIDs, true
}
