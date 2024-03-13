package index

import (
	"bytes"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetEvents tests that GetEvents returns the events in the correct order
func TestGetEvents(t *testing.T) {
	expectedEvents := make(flow.EventsList, 0, 6)
	expectedEvents = append(expectedEvents, generateTxEvents(unittest.IdentifierFixture(), 0, 1)...)
	expectedEvents = append(expectedEvents, generateTxEvents(unittest.IdentifierFixture(), 1, 3)...)
	expectedEvents = append(expectedEvents, generateTxEvents(unittest.IdentifierFixture(), 2, 2)...)

	storedEvents := make([]flow.Event, len(expectedEvents))
	copy(storedEvents, expectedEvents)

	// sort events in storage order (by tx ID)
	sort.Slice(storedEvents, func(i, j int) bool {
		cmp := bytes.Compare(storedEvents[i].TransactionID[:], storedEvents[j].TransactionID[:])
		if cmp == 0 {
			if storedEvents[i].TransactionIndex == storedEvents[j].TransactionIndex {
				return storedEvents[i].EventIndex < storedEvents[j].EventIndex
			}
			return storedEvents[i].TransactionIndex < storedEvents[j].TransactionIndex
		}
		return cmp < 0
	})

	events := storagemock.NewEvents(t)
	header := unittest.BlockHeaderFixture()

	events.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) ([]flow.Event, error) {
		return storedEvents, nil
	})

	eventsIndex := NewEventsIndex(events)
	err := eventsIndex.Initialize(&mockIndexReporter{})
	require.NoError(t, err)

	actualEvents, err := eventsIndex.ByBlockID(header.ID(), header.Height)
	require.NoError(t, err)

	// output events should be in the same order as the expected events
	assert.Len(t, actualEvents, len(expectedEvents))
	for i, event := range actualEvents {
		assert.Equal(t, expectedEvents[i], event)
	}
}

func generateTxEvents(txID flow.Identifier, txIndex uint32, count int) flow.EventsList {
	events := make(flow.EventsList, count)
	for i := 0; i < count; i++ {
		events[i] = flow.Event{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID,
			TransactionIndex: txIndex,
			EventIndex:       uint32(i),
		}
	}
	return events
}

type mockIndexReporter struct{}

func (r *mockIndexReporter) LowestIndexedHeight() (uint64, error) {
	return 0, nil
}

func (r *mockIndexReporter) HighestIndexedHeight() (uint64, error) {
	return math.MaxUint64, nil
}
