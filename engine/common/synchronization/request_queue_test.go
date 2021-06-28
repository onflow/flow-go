package synchronization

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestRequestQueue_Get tests that after pushing multiple items we can drain the queue using Get method
func TestRequestQueue_Get(t *testing.T) {
	q := NewRequestQueue(100)
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
