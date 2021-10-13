package synchronization

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRequestQueue_Get tests that after pushing multiple items we can drain the queue using Get method
func TestRequestQueue_Get(t *testing.T) {
	q := NewRequestHeap(100)
	items := 20
	messages := make(map[flow.Identifier]*engine.Message)
	for i := 0; i < items; i++ {
		msg := &engine.Message{
			OriginID: unittest.IdentifierFixture(),
			Payload:  unittest.IdentifierFixture(),
		}
		messages[msg.OriginID] = msg
		require.True(t, q.Put(msg))
	}

	for i := 0; i < items; i++ {
		msg, ok := q.Get()
		require.True(t, ok)
		expected, ok := messages[msg.OriginID]
		require.True(t, ok)
		require.Equal(t, expected, msg)
	}

	// at this point queue should be empty
	_, ok := q.Get()
	require.False(t, ok)
}

// TestRequestQueue_Put tests that putting an item into queue overwrites previous one
func TestRequestQueue_Put(t *testing.T) {
	q := NewRequestHeap(100)
	msg := &engine.Message{
		OriginID: unittest.IdentifierFixture(),
		Payload:  unittest.IdentifierFixture(),
	}
	require.True(t, q.Put(msg))
	require.Equal(t, msg, q.requests[msg.OriginID])

	newMsg := &engine.Message{
		OriginID: msg.OriginID,
		Payload:  unittest.BlockFixture(),
	}

	// this should overwrite message
	require.True(t, q.Put(newMsg))
	require.Equal(t, newMsg, q.requests[msg.OriginID])
}

// TestRequestQueue_PutAtMaxCapacity tests that putting an item over max capacity results in successful eject and put
func TestRequestQueue_PutAtMaxCapacity(t *testing.T) {
	limit := uint(10)
	q := NewRequestHeap(limit)
	messages := make(map[flow.Identifier]*engine.Message)
	for i := uint(0); i < limit; i++ {
		msg := &engine.Message{
			OriginID: unittest.IdentifierFixture(),
			Payload:  unittest.IdentifierFixture(),
		}
		require.True(t, q.Put(msg))
		messages[msg.OriginID] = msg
	}
	newMsg := &engine.Message{
		OriginID: unittest.IdentifierFixture(),
		Payload:  unittest.BlockFixture(),
	}
	require.True(t, q.Put(newMsg))

	ejectedCount := 0
	for {
		m, ok := q.Get()
		if !ok {
			break
		}
		expectedMessage, found := messages[m.OriginID]
		if !found {
			ejectedCount++
		} else {
			require.Equal(t, m, expectedMessage)
		}
	}
	require.Equal(t, 1, ejectedCount)
}
