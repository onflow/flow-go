package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestChunkDataPackRequestQueue_Sequential(t *testing.T) {
	sizeLimit := 100
	q := NewChunkDataPackRequestQueue(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	// initially queue must be zero
	require.Zero(t, q.Size())

	// initially there should be nothing to pop
	request, ok := q.Pop()
	require.False(t, ok)
	require.Nil(t, request)

	// initially there should be no head
	request, ok = q.Head()
	require.False(t, ok)
	require.Nil(t, request)

	requests := chunkDataRequestListFixture(sizeLimit)
	// pushing requests sequentially.
	for i, req := range requests {
		require.True(t, q.Push(req.ChunkId, req.RequesterId))

		// duplicate push should fail
		require.False(t, q.Push(req.ChunkId, req.RequesterId))

		// head should always point to the first element
		head, ok := q.Head()
		require.True(t, ok)
		require.Equal(t, head.ChunkId, requests[0].ChunkId)
		require.Equal(t, head.RequesterId, requests[0].RequesterId)

		require.Equal(t, q.Size(), uint(i+1))
	}

	// once queue meets the size limit, any extra push should fail.
	for i := 0; i < 100; i++ {
		require.False(t, q.Push(unittest.IdentifierFixture(), unittest.IdentifierFixture()))
	}

	// pop-ing requests sequentially.
	for i, req := range requests {
		popedReq, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, req.RequesterId, popedReq.RequesterId)
		require.Equal(t, req.ChunkId, popedReq.ChunkId)

		if i < len(requests)-1 {
			// queue is not empty yet.
			// head should be updated per pop (next element).
			head, ok := q.Head()
			require.True(t, ok, i)
			require.Equal(t, head.ChunkId, requests[i+1].ChunkId)
			require.Equal(t, head.RequesterId, requests[i+1].RequesterId)
		} else {
			// queue is empty,
			// head should be nil.
			head, ok := q.Head()
			require.False(t, ok)
			require.Nil(t, head)
		}

		require.Equal(t, q.Size(), uint(len(requests)-i-1))
	}
}

func TestChunkDataPackRequestQueue_Concurrent(t *testing.T) {
	sizeLimit := 100
	q := NewChunkDataPackRequestQueue(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	// initially queue must be zero
	require.Zero(t, q.Size())

	// initially there should be nothing to pop
	request, ok := q.Pop()
	require.False(t, ok)
	require.Nil(t, request)

	// initially there should be no head
	request, ok = q.Head()
	require.False(t, ok)
	require.Nil(t, request)

	pushWG := &sync.WaitGroup{}
	pushWG.Add(sizeLimit)

	requests := chunkDataRequestListFixture(sizeLimit)
	// pushing requests concurrently.
	for _, req := range requests {
		req := req // suppress loop variable
		go func() {
			require.True(t, q.Push(req.ChunkId, req.RequesterId))
			pushWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, pushWG.Wait, 100*time.Millisecond, "could not push all requests on time")

	// once queue meets the size limit, any extra push should fail.
	pushWG.Add(sizeLimit)
	for i := 0; i < sizeLimit; i++ {
		go func() {
			require.False(t, q.Push(unittest.IdentifierFixture(), unittest.IdentifierFixture()))
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
			popedReq, ok := q.Pop()

			matchLock.Lock()
			matchAndRemove(t, requests, popedReq)
			matchLock.Unlock()

			require.True(t, ok)
			popWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, popWG.Wait, 100*time.Millisecond, "could not pop all requests on time")

	// queue is empty,
	// head should be nil.
	head, ok := q.Head()
	require.False(t, ok)
	require.Nil(t, head)

	// queue must be empty after pop-ing all
	require.Zero(t, q.Size())
}

func chunkDataRequestListFixture(count int) []*mempool.ChunkDataPackRequest {
	list := make([]*mempool.ChunkDataPackRequest, count)
	for i := 0; i < count; i++ {
		list[i] = &mempool.ChunkDataPackRequest{
			ChunkId:     unittest.IdentifierFixture(),
			RequesterId: unittest.IdentifierFixture(),
		}
	}

	return list
}

func matchAndRemove(t *testing.T, requests []*mempool.ChunkDataPackRequest, req *mempool.ChunkDataPackRequest) {
	for i, r := range requests {
		if r.ChunkId == req.ChunkId && r.RequesterId == req.RequesterId {
			// removes the matched request from the list
			requests = append(requests[:i], requests[i+1:]...)
			return
		}
	}

	// no request found in the list to match
	require.Fail(t, "could not find a match for request")
}
