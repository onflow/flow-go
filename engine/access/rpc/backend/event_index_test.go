package backend

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
	txID0 := unittest.IdentifierFixture()
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()
	expectedEvents := flow.EventsList{
		{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID0,
			TransactionIndex: 0,
			EventIndex:       0,
		},
		{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID1,
			TransactionIndex: 1,
			EventIndex:       0,
		},
		{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID1,
			TransactionIndex: 1,
			EventIndex:       1,
		},
		{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID1,
			TransactionIndex: 1,
			EventIndex:       2,
		},
		{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID2,
			TransactionIndex: 2,
			EventIndex:       0,
		},
		{
			Type:             unittest.EventTypeFixture(flow.Localnet),
			TransactionID:    txID2,
			TransactionIndex: 2,
			EventIndex:       1,
		},
	}

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
	eventsIndex.Initialize(&mockIndexReporter{})

	actualEvents, err := eventsIndex.GetEvents(header.ID(), header.Height)
	require.NoError(t, err)

	// output events should be in the same order as the expected events
	assert.Len(t, actualEvents, len(expectedEvents))
	for i, event := range actualEvents {
		assert.Equal(t, expectedEvents[i], event)
	}
}

type mockIndexReporter struct{}

func (r *mockIndexReporter) LowestIndexedHeight() (uint64, error) {
	return 0, nil
}

func (r *mockIndexReporter) HighestIndexedHeight() (uint64, error) {
	return math.MaxUint64, nil
}
