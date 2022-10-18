package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/queue/internal"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEntityRequestQueue_Sequential evaluates correctness of queue implementation against sequential push and pop.
func TestEntityRequestQueue_Sequential(t *testing.T) {
	sizeLimit := 100
	q := NewEntityRequestStore(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	// initially queue must be zero
	require.Zero(t, q.Size())

	// initially there should be nothing to pop
	originId, request, ok := q.pop()
	require.False(t, ok)
	require.Equal(t, originId, flow.Identifier{})
	require.Nil(t, request)

	requests := entityRequestListFixture(sizeLimit)
	// pushing requests sequentially.
	for i, req := range requests {
		require.True(t, q.push(requests[i].OriginId, req.EntityIDs))

		// duplicate push should fail
		require.False(t, q.push(requests[i].OriginId, req.EntityIDs))
		require.Equal(t, q.Size(), uint(i+1))
	}

	// once queue meets the size limit, any extra push should fail.
	extraRequests := entityRequestListFixture(100)
	for i := 0; i < 100; i++ {
		require.False(t, q.push(extraRequests[i].OriginId, extraRequests[i].EntityIDs))

		// size should not change
		require.Equal(t, q.Size(), uint(sizeLimit))
	}

	// pop-ing requests sequentially.
	for i, req := range requests {
		popedOriginId, popedReq, ok := q.pop()
		require.True(t, ok)
		require.Equal(t, req.EntityIDs, popedReq)
		require.Equal(t, req.OriginId, popedOriginId)

		require.Equal(t, q.Size(), uint(len(requests)-i-1))
	}
}

// TestEntityRequestsQueue_Concurrent evaluates correctness of queue implementation against concurrent push and pop.
func TestEntityRequestsQueue_Concurrent(t *testing.T) {
	sizeLimit := 100
	q := NewEntityRequestStore(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	// initially queue must be zero
	require.Zero(t, q.Size())

	// initially there should be nothing to pop
	originId, request, ok := q.pop()
	require.False(t, ok)
	require.Equal(t, originId, flow.Identifier{})
	require.Nil(t, request)

	pushWG := &sync.WaitGroup{}
	pushWG.Add(sizeLimit)

	requests := entityRequestListFixture(sizeLimit)
	// pushing requests concurrently.
	for _, req := range requests {
		req := req // suppress loop variable
		go func() {
			require.True(t, q.push(req.OriginId, req.EntityIDs))
			pushWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, pushWG.Wait, 100*time.Millisecond, "could not push all requests on time")

	// once queue meets the size limit, any extra push should fail.
	pushWG.Add(sizeLimit)
	for i := 0; i < sizeLimit; i++ {
		go func() {
			req := entityRequestFixture()
			require.False(t, q.push(req.OriginId, req.EntityIDs))
			pushWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, pushWG.Wait, 100*time.Millisecond, "could not push all requests on time")

	popWG := &sync.WaitGroup{}
	popWG.Add(sizeLimit)
	matchLock := &sync.Mutex{}

	// pop-ing requests concurrently.
	for i := 0; i < sizeLimit; i++ {
		go func() {
			popedOriginId, popedReq, ok := q.pop()
			require.True(t, ok)

			matchLock.Lock()
			matchAndRemoveEntityRequest(t, requests, popedOriginId, popedReq)
			matchLock.Unlock()

			popWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, popWG.Wait, 100*time.Millisecond, "could not pop all requests on time")

	// queue must be empty after pop-ing all
	require.Zero(t, q.Size())
}

func entityRequestListFixture(count int) []*internal.RequestEntity {
	list := make([]*internal.RequestEntity, count)
	for i := 0; i < count; i++ {
		list[i] = entityRequestFixture()
	}

	return list
}

func entityRequestFixture() *internal.RequestEntity {
	req := internal.NewRequestEntity(unittest.IdentifierFixture(), unittest.IdentifierListFixture(10))
	return &req
}

// matchAndRemoveEntityRequest checks existence of the request in the "requests" array and "originId" in the corresponding place of "originIds" array.
// If a match is found, it is removed.
// If no match is found for a request, it fails the test.
func matchAndRemoveEntityRequest(t *testing.T, requests []*internal.RequestEntity, originId flow.Identifier, entityIds []flow.Identifier) []*internal.RequestEntity {
	for i, r := range requests {
		if r.OriginId == originId {
			require.ElementsMatch(t, r.EntityIDs, entityIds)
			// removes the matched request from the list
			if i == len(requests)-1 {
				requests = requests[:i]
			} else {
				requests = append(requests[:i], requests[i+1:]...)
			}

			return requests
		}
	}

	// no request found in the list to match
	require.Fail(t, "could not find a match for request")

	return requests
}
