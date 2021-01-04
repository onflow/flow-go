package fvm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
)

func Test_GenerateUUID(t *testing.T) {
	ledger := state.NewMapLedger()
	st := state.NewState(ledger)
	uuidsA := state.NewUUIDs(st)
	genA := fvm.NewUUIDGenerator(uuidsA)

	uuidA, err := genA.GenerateUUID()
	require.NoError(t, err)
	uuidB, err := genA.GenerateUUID()
	require.NoError(t, err)
	uuidC, err := genA.GenerateUUID()
	require.NoError(t, err)

	require.Equal(t, uint64(0), uuidA)
	require.Equal(t, uint64(1), uuidB)
	require.Equal(t, uint64(2), uuidC)

	// Create new generator instance from same ledger
	uuidsB := state.NewUUIDs(st)
	genB := fvm.NewUUIDGenerator(uuidsB)

	uuidD, err := genB.GenerateUUID()
	require.NoError(t, err)
	uuidE, err := genB.GenerateUUID()
	require.NoError(t, err)
	uuidF, err := genB.GenerateUUID()
	require.NoError(t, err)

	require.Equal(t, uint64(3), uuidD)
	require.Equal(t, uint64(4), uuidE)
	require.Equal(t, uint64(5), uuidF)
}
