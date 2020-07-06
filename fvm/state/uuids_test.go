package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/fvm/state"
)

func TestUUIDs_GetAndSetUUID(t *testing.T) {
	ledger := state.NewMapLedger()

	uuidsA := state.NewUUIDs(ledger)

	uuid, err := uuidsA.GetUUID() // start from zero
	require.NoError(t, err)

	require.Equal(t, uint64(0), uuid)
	uuidsA.SetUUID(5)

	// create new UUIDs instance
	uuidsB := state.NewUUIDs(ledger)
	uuid, err = uuidsB.GetUUID() // should read saved value
	require.NoError(t, err)

	require.Equal(t, uint64(5), uuid)
}
