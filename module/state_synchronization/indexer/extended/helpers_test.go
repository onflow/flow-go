package extended

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// mockHeightProvider is a simple mock implementing the heightProvider interface.
type mockHeightProvider struct {
	latestHeight    uint64
	latestHeightErr error
	firstHeight     uint64
	isInitialized   bool
}

func (m *mockHeightProvider) LatestIndexedHeight() (uint64, error) {
	return m.latestHeight, m.latestHeightErr
}

func (m *mockHeightProvider) UninitializedFirstHeight() (uint64, bool) {
	return m.firstHeight, m.isInitialized
}

// ===== TestNextHeight =====

func TestNextHeight(t *testing.T) {
	t.Parallel()

	t.Run("initialized store returns latestHeight+1", func(t *testing.T) {
		store := &mockHeightProvider{
			latestHeight:    99,
			latestHeightErr: nil,
		}

		height, err := nextHeight(store)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), height)
	})

	t.Run("uninitialized store returns firstHeight", func(t *testing.T) {
		store := &mockHeightProvider{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     42,
			isInitialized:   false,
		}

		height, err := nextHeight(store)
		require.NoError(t, err)
		assert.Equal(t, uint64(42), height)
	})

	t.Run("unexpected error from LatestIndexedHeight propagates", func(t *testing.T) {
		unexpectedErr := fmt.Errorf("disk I/O error")
		store := &mockHeightProvider{
			latestHeightErr: unexpectedErr,
		}

		_, err := nextHeight(store)
		require.Error(t, err)
		assert.ErrorIs(t, err, unexpectedErr)
	})

	t.Run("inconsistent state: not bootstrapped but isInitialized returns error", func(t *testing.T) {
		store := &mockHeightProvider{
			latestHeightErr: storage.ErrNotBootstrapped,
			firstHeight:     42,
			isInitialized:   true,
		}

		_, err := nextHeight(store)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "but index is initialized")
	})
}

// ===== TestFlattenEvents =====

func TestFlattenEvents(t *testing.T) {
	t.Parallel()

	t.Run("nil map returns nil", func(t *testing.T) {
		result := flattenEvents(nil)
		assert.Nil(t, result)
	})

	t.Run("empty map returns nil", func(t *testing.T) {
		result := flattenEvents(map[uint32][]flow.Event{})
		assert.Nil(t, result)
	})

	t.Run("single tx single event", func(t *testing.T) {
		event := flow.Event{TransactionIndex: 0, EventIndex: 0, Type: "A.Test.Foo"}
		result := flattenEvents(map[uint32][]flow.Event{
			0: {event},
		})
		require.Len(t, result, 1)
		assert.Equal(t, event, result[0])
	})

	t.Run("single tx multiple events preserves order", func(t *testing.T) {
		event0 := flow.Event{TransactionIndex: 0, EventIndex: 0, Type: "A.Test.First"}
		event1 := flow.Event{TransactionIndex: 0, EventIndex: 1, Type: "A.Test.Second"}
		event2 := flow.Event{TransactionIndex: 0, EventIndex: 2, Type: "A.Test.Third"}
		result := flattenEvents(map[uint32][]flow.Event{
			0: {event0, event1, event2},
		})
		require.Len(t, result, 3)
		assert.Equal(t, event0, result[0])
		assert.Equal(t, event1, result[1])
		assert.Equal(t, event2, result[2])
	})

	t.Run("multiple txs multiple events all included", func(t *testing.T) {
		eventsA := []flow.Event{
			{TransactionIndex: 0, EventIndex: 0, Type: "A.Test.A0"},
			{TransactionIndex: 0, EventIndex: 1, Type: "A.Test.A1"},
		}
		eventsB := []flow.Event{
			{TransactionIndex: 1, EventIndex: 0, Type: "A.Test.B0"},
		}
		eventsC := []flow.Event{
			{TransactionIndex: 2, EventIndex: 0, Type: "A.Test.C0"},
			{TransactionIndex: 2, EventIndex: 1, Type: "A.Test.C1"},
			{TransactionIndex: 2, EventIndex: 2, Type: "A.Test.C2"},
		}
		result := flattenEvents(map[uint32][]flow.Event{
			0: eventsA,
			1: eventsB,
			2: eventsC,
		})
		require.Len(t, result, 6)

		// Map iteration order is non-deterministic, so verify all events are present
		// by checking set membership rather than exact ordering across tx groups.
		eventTypes := make(map[flow.EventType]bool)
		for _, e := range result {
			eventTypes[e.Type] = true
		}
		assert.True(t, eventTypes["A.Test.A0"])
		assert.True(t, eventTypes["A.Test.A1"])
		assert.True(t, eventTypes["A.Test.B0"])
		assert.True(t, eventTypes["A.Test.C0"])
		assert.True(t, eventTypes["A.Test.C1"])
		assert.True(t, eventTypes["A.Test.C2"])
	})

	t.Run("events order is preserved within each tx group", func(t *testing.T) {
		events := []flow.Event{
			{TransactionIndex: 5, EventIndex: 0, Type: "A.Test.First"},
			{TransactionIndex: 5, EventIndex: 1, Type: "A.Test.Second"},
			{TransactionIndex: 5, EventIndex: 2, Type: "A.Test.Third"},
		}
		result := flattenEvents(map[uint32][]flow.Event{
			5: events,
		})
		require.Len(t, result, 3)
		for i, e := range result {
			assert.Equal(t, events[i], e)
		}
	})
}
