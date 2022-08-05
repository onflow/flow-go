package queue

import (
	"testing"

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
	require.False(t, q.Push(unittest.IdentifierFixture(), unittest.IdentifierFixture()))
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
