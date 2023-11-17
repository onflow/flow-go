package execution

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	state_synchronization "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestInitDataAvailable(t *testing.T) {
	rootBlockHeight := uint64(1)
	registersAsync := NewRegistersAsyncStore()
	// test data available on init
	registerID := unittest.RegisterIDFixture()
	invalidRegisterID := flow.RegisterID{
		Owner: "ha",
		Key:   "ha",
	}
	registerValue1 := []byte("response1")
	registerValue2 := []byte("response2")
	require.False(t, registersAsync.initialized.Load())
	firstHeight := rootBlockHeight
	latestHeight := rootBlockHeight + 1

	registers := storagemock.NewRegisterIndex(t)
	registers.On("Get", registerID, firstHeight).Return(registerValue1, nil)
	registers.On("Get", registerID, latestHeight).Return(registerValue2, nil)
	registers.On("Get", invalidRegisterID, latestHeight).Return(nil, storage.ErrNotFound)

	indexReporter := state_synchronization.NewIndexReporter(t)
	indexReporter.On("LowestIndexedHeight").Return(firstHeight)
	indexReporter.On("HighestIndexedHeight").Return(latestHeight)

	// registersDB bootstrapped, correct values returned
	registersAsync.InitDataAvailable(indexReporter, registers)
	require.True(t, registersAsync.initialized.Load())
	val1, err := registersAsync.RegisterValues([]flow.RegisterID{registerID}, firstHeight)
	require.NoError(t, err)
	require.Equal(t, val1[0], registerValue1)

	val2, err := registersAsync.RegisterValues([]flow.RegisterID{registerID}, latestHeight)
	require.NoError(t, err)
	require.Equal(t, val2[0], registerValue2)

	// out of bounds height, correct error returned
	_, err = registersAsync.RegisterValues([]flow.RegisterID{registerID}, latestHeight+1)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)

	// no register value available, correct error returned
	_, err = registersAsync.RegisterValues([]flow.RegisterID{invalidRegisterID}, latestHeight)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestRegisterValuesDataUnAvailable(t *testing.T) {
	rootBlockHeight := uint64(1)
	registersAsync := NewRegistersAsyncStore()
	// registerDB not bootstrapped, correct error returned
	registerID := unittest.RegisterIDFixture()
	require.False(t, registersAsync.initialized.Load())
	_, err := registersAsync.RegisterValues([]flow.RegisterID{registerID}, rootBlockHeight)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
}
