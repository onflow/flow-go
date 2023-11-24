package execution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitDataAvailable(t *testing.T) {
	rootBlockHeight := uint64(1)
	// test data available on init
	registerID := unittest.RegisterIDFixture()
	invalidRegisterID := flow.RegisterID{
		Owner: "ha",
		Key:   "ha",
	}
	registerValue1 := []byte("response1")
	registerValue2 := []byte("response2")
	firstHeight := rootBlockHeight
	latestHeight := rootBlockHeight + 1

	t.Parallel()

	t.Run("registersDB bootstrapped correct values returned", func(t *testing.T) {
		registersAsync := NewRegistersAsyncStore()
		require.False(t, registersAsync.initialized.Load())
		registers := storagemock.NewRegisterIndex(t)
		registers.On("Get", registerID, firstHeight).Return(registerValue1, nil)
		registers.On("Get", registerID, latestHeight).Return(registerValue2, nil)
		registers.On("FirstHeight").Return(firstHeight)
		registers.On("LatestHeight").Return(latestHeight)

		registersAsync.InitDataAvailable(registers)
		require.True(t, registersAsync.initialized.Load())
		val1, err := registersAsync.RegisterValues([]flow.RegisterID{registerID}, firstHeight)
		require.NoError(t, err)
		require.Equal(t, val1[0], registerValue1)

		val2, err := registersAsync.RegisterValues([]flow.RegisterID{registerID}, latestHeight)
		require.NoError(t, err)
		require.Equal(t, val2[0], registerValue2)
	})

	t.Run("out of bounds height correct error returned", func(t *testing.T) {
		registersAsync := NewRegistersAsyncStore()
		require.False(t, registersAsync.initialized.Load())
		registers := storagemock.NewRegisterIndex(t)
		registers.On("LatestHeight").Return(latestHeight)

		registersAsync.InitDataAvailable(registers)
		require.True(t, registersAsync.initialized.Load())
		_, err := registersAsync.RegisterValues([]flow.RegisterID{registerID}, latestHeight+1)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
	})

	t.Run("no register value available correct error returned", func(t *testing.T) {
		registersAsync := NewRegistersAsyncStore()
		require.False(t, registersAsync.initialized.Load())
		registers := storagemock.NewRegisterIndex(t)
		registers.On("Get", invalidRegisterID, latestHeight).Return(nil, storage.ErrNotFound)
		registers.On("FirstHeight").Return(firstHeight)
		registers.On("LatestHeight").Return(latestHeight)

		registersAsync.InitDataAvailable(registers)
		require.True(t, registersAsync.initialized.Load())
		_, err := registersAsync.RegisterValues([]flow.RegisterID{invalidRegisterID}, latestHeight)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
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
