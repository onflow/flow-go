// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"math/rand"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// EjectFunc is a function used to pick an entity to evict from the memory pool
// backend when it overflows its limit. A custom eject function can be injected
// into the memory pool upon creation, which allows us to hook into the eject
// to clean up auxiliary data and/or to change the strategy of eviction.
type EjectFunc func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity)

// EjectFakeRandom relies on the random map iteration in Go to pick the entity we eject
// from the entity set. It picks the first entity upon iteration, thus being the fastest
// way to pick an entity to be evicted; at the same time, it conserves the random bias
// of the Go map iteration.
func EjectFakeRandom(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var entityID flow.Identifier
	var entity flow.Entity
	for entityID, entity = range entities {
		break
	}
	return entityID, entity
}

// EjectTrueRandom relies on a random generator to pick a random entity to eject from the
// entity set. It will, on average, iterate through half the entities of the set. However,
// it provides us with a truly evenly distributed random selection.
func EjectTrueRandom(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	var entityID flow.Identifier
	var entity flow.Entity
	i := 0
	n := rand.Intn(len(entities))
	for entityID, entity = range entities {
		if i == n {
			break
		}
		i++
	}
	return entityID, entity
}

// EjectPanic simply panics, crashing the program. Useful when cache is not expected
// to grow beyond certain limits, but ejecting is not applicable
func EjectPanic(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	panic("unexpected: mempool size over the limit")
}

// QueueEjector provides a swift FIFO ejection functionality
type QueueEjector struct {
	sync.Mutex
	queue chan flow.Identifier
}

func NewQueueEjector(limit uint) *QueueEjector {
	return &QueueEjector{
		queue: make(chan flow.Identifier, limit),
	}
}

// Push should be called every time a new entity is added to the mempool.
// It enqueues the entity for later ejection.
func (q *QueueEjector) Push(entityID flow.Identifier) {
	q.Lock()
	defer q.Unlock()
	if len(q.queue) < cap(q.queue) {
		q.queue <- entityID
	}
}

// Eject is the EjectFunc of QueueEjector. It dequeues an identifier and returns its corresponding entity.
// In case the dequeued identifier of entity does not exist, it returns a random entity of the queue.
func (q *QueueEjector) Eject(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	q.Lock()
	defer q.Unlock()

	if len(q.queue) == 0 {
		return EjectTrueRandom(entities)
	}

	id := <-q.queue

	entity, ok := entities[id]

	if !ok {
		return EjectTrueRandom(entities)
	}

	return id, entity
}
