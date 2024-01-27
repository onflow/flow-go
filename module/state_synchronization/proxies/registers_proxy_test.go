package proxies

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitialize(t *testing.T) {
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
		registerStore := NewRegistersStore()
		registers := storagemock.NewRegisterIndex(t)
		registers.On("Get", registerID, firstHeight).Return(registerValue1, nil)
		registers.On("Get", registerID, latestHeight).Return(registerValue2, nil)
		registers.On("FirstHeight").Return(firstHeight)
		registers.On("LatestHeight").Return(latestHeight)

		require.NoError(t, registerStore.Initialize(registers))
		val1, err := registerStore.RegisterValues([]flow.RegisterID{registerID}, firstHeight)
		require.NoError(t, err)
		require.Equal(t, val1[0], registerValue1)

		val2, err := registerStore.RegisterValues([]flow.RegisterID{registerID}, latestHeight)
		require.NoError(t, err)
		require.Equal(t, val2[0], registerValue2)
	})

	t.Run("out of bounds height correct error returned", func(t *testing.T) {
		registerStore := NewRegistersStore()
		registers := storagemock.NewRegisterIndex(t)
		registers.On("LatestHeight").Return(latestHeight)

		require.NoError(t, registerStore.Initialize(registers))
		_, err := registerStore.RegisterValues([]flow.RegisterID{registerID}, latestHeight+1)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
	})

	t.Run("no register value available correct error returned", func(t *testing.T) {
		registerStore := NewRegistersStore()
		registers := storagemock.NewRegisterIndex(t)
		registers.On("Get", invalidRegisterID, latestHeight).Return(nil, storage.ErrNotFound)
		registers.On("FirstHeight").Return(firstHeight)
		registers.On("LatestHeight").Return(latestHeight)

		require.NoError(t, registerStore.Initialize(registers))
		_, err := registerStore.RegisterValues([]flow.RegisterID{invalidRegisterID}, latestHeight)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestRegisterValuesDataUnAvailable(t *testing.T) {
	rootBlockHeight := uint64(1)
	registerStore := NewRegistersStore()
	// registerDB not bootstrapped, correct error returned
	registerID := unittest.RegisterIDFixture()
	_, err := registerStore.RegisterValues([]flow.RegisterID{registerID}, rootBlockHeight)
	require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
}

func TestInitDataRepeatedCalls(t *testing.T) {
	registerStore := NewRegistersStore()
	registers1 := storagemock.NewRegisterIndex(t)
	registers2 := storagemock.NewRegisterIndex(t)

	require.NoError(t, registerStore.Initialize(registers1))
	require.Error(t, registerStore.Initialize(registers2))
}
