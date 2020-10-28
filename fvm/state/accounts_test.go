package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccounts_Create_SetsRegisters(t *testing.T) {
	t.Run("Sets registers", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		require.Equal(t, len(ledger.RegisterTouches), 4) // exists + code + key count
	})

	t.Run("Fails if account exists", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.Create(nil, address)

		require.Error(t, err)
	})
}

func TestAccounts_GetWithNoKeys(t *testing.T) {
	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func TestAccounts_GetWithNoKeysCounter(t *testing.T) {
	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	ledger.Delete(
		string(address.Bytes()),
		string(address.Bytes()),
		"public_key_count")

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}
