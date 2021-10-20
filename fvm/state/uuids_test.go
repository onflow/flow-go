package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func TestUUIDs_GetAndSetUUID(t *testing.T) {
	view := utils.NewSimpleView()
	sth := state.NewStateHolder(state.NewState(view, state.NewInteractionLimiter(state.WithInteractionLimit(false))))
	uuidsA := state.NewUUIDGenerator(sth)

	uuid, err := uuidsA.GetUUID() // start from zero
	require.NoError(t, err)
	require.Equal(t, uint64(0), uuid)

	err = uuidsA.SetUUID(5)
	require.NoError(t, err)

	// create new UUIDs instance
	uuidsB := state.NewUUIDGenerator(sth)
	uuid, err = uuidsB.GetUUID() // should read saved value
	require.NoError(t, err)

	require.Equal(t, uint64(5), uuid)
}

func Test_GenerateUUID(t *testing.T) {
	view := utils.NewSimpleView()
	sth := state.NewStateHolder(state.NewState(view, state.NewInteractionLimiter(state.WithInteractionLimit(false))))
	genA := state.NewUUIDGenerator(sth)

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
	genB := state.NewUUIDGenerator(sth)

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
