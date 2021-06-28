package synchronization

import (
	"github.com/onflow/flow-go/model/flow"
	"sync"
)

type RequestQueue struct {
	lock     sync.Mutex
	limit    uint
	requests map[flow.Identifier]interface{}
}

func NewRequestQueue(limit uint) *RequestQueue {
	return &RequestQueue{
		limit:    limit,
		requests: make(map[flow.Identifier]interface{}),
	}
}

func (q *RequestQueue) Push(originID flow.Identifier, req interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.requests[originID] = req
	q.reduce()
}

func (q *RequestQueue) Pop() (flow.Identifier, interface{}, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	var originID flow.Identifier
	var req interface{}

	if len(q.requests) == 0 {
		return originID, req, false
	}

	// pick first element using go map randomness property
	for originID, req = range q.requests {
		break
	}

	return originID, req, true
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
