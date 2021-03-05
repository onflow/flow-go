package signature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
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

	epochQuery := NewEpochQuery(t, 101)
	for _, e := range epochs {
		epochQuery.Add(e)
	}

	snapshot := new(mockprotocol.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)

	state := new(mockprotocol.State)
	state.On("Final").Return(snapshot)

	signerStore := NewEpochAwareSignerStore(state, new(mockstorage.DKGKeys))

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
		e, err := signerStore.epochForView(tc.view)
		require.NoError(t, err)
		require.Equal(t, tc.epoch, e)
	}
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/
// cant reuse the EpochQuery from the unittest package because of a dependency
// cycle

// EpochQuery implements protocol.EpochQuery for testing purposes.
type EpochQuery struct {
	t         *testing.T
	counter   uint64                    // represents the current epoch
	byCounter map[uint64]protocol.Epoch // all epochs
}

func NewEpochQuery(t *testing.T, counter uint64) *EpochQuery {
	mock := &EpochQuery{
		t:         t,
		counter:   counter,
		byCounter: make(map[uint64]protocol.Epoch),
	}
	return mock
}

func (mock *EpochQuery) Current() protocol.Epoch {
	return mock.byCounter[mock.counter]
}

func (mock *EpochQuery) Next() protocol.Epoch {
	return mock.byCounter[mock.counter+1]
}

func (mock *EpochQuery) Previous() protocol.Epoch {
	return mock.byCounter[mock.counter-1]
}

func (mock *EpochQuery) Add(epoch protocol.Epoch) {
	counter, err := epoch.Counter()
	assert.Nil(mock.t, err, "cannot add epoch with invalid counter")
	mock.byCounter[counter] = epoch
}
