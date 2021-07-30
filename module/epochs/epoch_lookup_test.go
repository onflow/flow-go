package epochs

import (
	"testing"

	"github.com/stretchr/testify/require"

	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestEpochForView(t *testing.T) {
	// epoch 100: 0    -> 999
	// epoch 101: 1000 -> 1999
	// epoch 102: 2000 -> 2999
	epochs := []*mockprotocol.Epoch{}
	for i := 0; i < 3; i++ {
		firstView := i * 1000
		lastView := i*1000 + 999
		counter := 100 + i
		epoch := new(mockprotocol.Epoch)
		epoch.On("FirstView").Return(uint64(firstView), nil)
		epoch.On("FinalView").Return(uint64(lastView), nil)
		epoch.On("Counter").Return(uint64(counter), nil)
		epochs = append(epochs, epoch)
	}

	epochQuery := mocks.NewEpochQuery(t, 101)
	for _, e := range epochs {
		epochQuery.Add(e)
	}

	snapshot := new(mockprotocol.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)

	state := new(mockprotocol.State)
	state.On("Final").Return(snapshot)

	lookup := NewEpochLookup(state)

	type testCase struct {
		view  uint64
		epoch uint64
	}

	testCases := []testCase{
		{view: 0, epoch: 100},
		{view: 500, epoch: 100},
		{view: 999, epoch: 100},
		{view: 1000, epoch: 101},
		{view: 1500, epoch: 101},
		{view: 1999, epoch: 101},
		{view: 2000, epoch: 102},
		{view: 2500, epoch: 102},
		{view: 2999, epoch: 102},
	}

	for _, tc := range testCases {
		e, err := lookup.EpochForView(tc.view)
		require.NoError(t, err)
		require.Equal(t, tc.epoch, e)
	}
}
