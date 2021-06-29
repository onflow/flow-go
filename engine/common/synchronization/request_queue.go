package synchronization

import (
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// RequestQueue is a special queue that implements engine.MessageStore interface and
// indexes requests by originator. If request will be sent by same originator then it will replace the old one.
// Comparing to default FIFO queue this one can contain MAX one request for origin ID.
// Getting value from queue as well as ejecting is pseudo-random.
type RequestQueue struct {
	lock     sync.Mutex
	limit    uint
	requests map[flow.Identifier]*engine.Message
}

func NewRequestQueue(limit uint) *RequestQueue {
	return &RequestQueue{
		limit:    limit,
		requests: make(map[flow.Identifier]*engine.Message),
	}
}

// Put stores message into requests map using OriginID as key.
// Returns always true
func (q *RequestQueue) Put(message *engine.Message) bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	// first try to eject if we are at max capacity, we need to do this way
	// to prevent a situation where just inserted item gets ejected
	q.reduce()
	// at this point we can be sure that there is at least one slot
	q.requests[message.OriginID] = message
	return true
}

// Get returns pseudo-random element from request storage using go map properties.
func (q *RequestQueue) Get() (*engine.Message, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	var originID flow.Identifier
	var msg *engine.Message

	if len(q.requests) == 0 {
		return nil, false
	}

	// pick first element using go map randomness property
	for originID, msg = range q.requests {
		break
	}

	delete(q.requests, originID)

	return msg, true
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit. If called on max capacity will eject at least one element.
func (q *RequestQueue) reduce() {

	// we keep reducing the cache size until we are at limit again
	for len(q.requests) >= int(q.limit) {

		// eject first element using go map properties
		var key flow.Identifier
		for originID := range q.requests {
			key = originID
			break
		}

		delete(q.requests, key)
	}
}
