package persisters

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegistersPersister_PersistWithEmptyData(t *testing.T) {
	t.Parallel()

	height := uint64(1000)
	storedRegisters := make(flow.RegisterEntries, 3)
	for i := range storedRegisters {
		storedRegisters[i] = unittest.RegisterEntryFixture()
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		registers := storagemock.NewRegisterIndex(t)
		registers.On("Store", storedRegisters, height).Return(nil).Once()

		persister := NewRegistersPersister(storedRegisters, registers, height)

		err := persister.Persist()
		require.NoError(t, err)
	})

	// Registers must be stored for every height, even if empty
	t.Run("persist empty registers", func(t *testing.T) {
		t.Parallel()

		storedRegisters := make(flow.RegisterEntries, 0)

		registers := storagemock.NewRegisterIndex(t)
		registers.On("Store", storedRegisters, height).Return(nil).Once()

		persister := NewRegistersPersister(storedRegisters, registers, height)

		err := persister.Persist()
		require.NoError(t, err)
	})

	t.Run("persist error", func(t *testing.T) {
		t.Parallel()

		expectedErr := fmt.Errorf("test error")

		registers := storagemock.NewRegisterIndex(t)
		registers.On("Store", storedRegisters, height).Return(expectedErr).Once()

		persister := NewRegistersPersister(storedRegisters, registers, height)

		err := persister.Persist()
		require.ErrorIs(t, err, expectedErr)
	})
}
