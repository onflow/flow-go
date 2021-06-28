package synchronization

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"sync"
)

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

func (q *RequestQueue) Put(message *engine.Message) bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.requests[message.OriginID] = message
	q.reduce()
}

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
// configured memory pool size limit.
func (q *RequestQueue) reduce() {

	// we keep reducing the cache size until we are at limit again
	for len(q.requests) > int(q.limit) {

		// eject first element using go map properties
		var key flow.Identifier
		for originID := range q.requests {
			key = originID
			break
		}

		delete(q.requests, key)
	}
}
