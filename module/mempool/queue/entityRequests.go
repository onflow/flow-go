package queue

import (
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
)

// EntityRequestStore is a FIFO (first-in-first-out) size-bound queue for maintaining EntityRequests.
// It is designed to be utilized at the common ProviderEngine to maintain and respond entity requests.
type EntityRequestStore struct {
	mu        sync.RWMutex
	cache     *herocache.Cache
	sizeLimit uint
}

var _ mempool.EntityRequestStore = (*EntityRequestStore)(nil)

func (e *EntityRequestStore) Put(message *engine.Message) bool {
	return e.push(message.OriginID, message.Payload.(*messages.EntityRequest))
}

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
func (e *EntityRequestStore) push(originId flow.Identifier, request *messages.EntityRequest) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cache.Size() >= e.sizeLimit {
		// we check size before attempt on a push,
		// although HeroCache is on no-ejection mode and discards pushes beyond limit,
		// we save an id computation by just checking the size here.
		return false
	}

	req := requestEntity{
		EntityRequest: *request, // hero cache does not support pointer types for sake of heap optimization.
		originId:      originId,
		id:            identifierOfRequest(originId, request),
	}

	return e.cache.Add(req.id, req)
}

// pop removes and returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., poping an empty queue returns false.
func (e *EntityRequestStore) pop() (flow.Identifier, *messages.EntityRequest, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	head, ok := e.cache.Head()
	if !ok {
		// cache is empty, and there is no head yet to pop.
		return flow.Identifier{}, nil, false
	}

	e.cache.Remove(head.ID())
	request := head.(requestEntity)
	return request.originId, &request.EntityRequest, true
}

// requestEntity is a wrapper around EntityRequest that implements Entity interface for it, and
// also internally caches its identifier.
type requestEntity struct {
	messages.EntityRequest

	// identifier of the requester.
	originId flow.Identifier

	// caching identifier to avoid cpu overhead per query.
	id flow.Identifier
}

func (r requestEntity) ID() flow.Identifier {
	return r.id
}

func (r requestEntity) Checksum() flow.Identifier {
	return r.id
}

func identifierOfRequest(originId flow.Identifier, request *messages.EntityRequest) flow.Identifier {
	requestIds := flow.MakeID(request.EntityIDs)
	return flow.MakeID(append(originId[:], requestIds[:]...))
}
