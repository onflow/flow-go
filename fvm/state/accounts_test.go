package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestAccounts_GetWithNoKeys(t *testing.T) {
	chain := flow.Mainnet.Chain()

	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger, chain)

	address, err := accounts.Create(nil)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func TestAccounts_GetWithNoKeysCounter(t *testing.T) {
	chain := flow.Mainnet.Chain()

	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger, chain)

	address, err := accounts.Create(nil)
	require.NoError(t, err)

	countRegister := state.RegisterID(
		string(address.Bytes()),
		string(address.Bytes()),
		"public_key_count",
	)

	ledger.Delete(countRegister)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}
