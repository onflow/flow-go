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
	var lastMsg *engine.Message // tracks the last message that we added to the heap
	for i := uint(0); i < limit+1; i++ {
		lastMsg = &engine.Message{
			OriginID: unittest.IdentifierFixture(),
			Payload:  unittest.IdentifierFixture(),
		}
		messages[lastMsg.OriginID] = lastMsg
		require.True(t, q.Put(lastMsg))
	}

	// We have inserted 11 elements into the heap with capacity 10. The heap should now store
	// 10 of these elements. By convention, the last-inserted element should be stored (no
	// ejecting the just stored element).
	lastMessagePopped := false
	for k := uint(0); k < limit; k++ {
		m, ok := q.Get()
		require.True(t, ok)

		// in the following code segment, we check that:
		//  - only previously inserted elements are popped from the heap
		//  - each element is only popped once
		expectedMessage, found := messages[m.OriginID]
		require.True(t, found)
		require.Equal(t, m, expectedMessage)
		delete(messages, m.OriginID)

		// We expect that the most-recently inserted element is popped from the heap eventually,
		// because this element has zero probability of being ejected itself
		if m == lastMsg {
			lastMessagePopped = true
		}
	}

	// We have popped 10 elements from a heap with capacity 10, so the heap should now be empty:
	m, ok := q.Get()
	require.False(t, ok)
	require.Nil(t, m)

	// We have removed 10 out of 11 elements from `messages`. What remains in the map
	// is that one message that the heap ejected. By convention, the heap should always eject first
	// before accepting a new element. Therefore, the last-inserted element ('lastMsg') should have
	// been popped from the heap in the for-loop, i.e. `lastMessagePopped` is expected to be true.
	require.Equal(t, 1, len(messages))
	require.True(t, lastMessagePopped)
}
