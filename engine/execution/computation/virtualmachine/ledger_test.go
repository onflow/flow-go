package virtualmachine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

func Test_AccountWithNoKeys(t *testing.T) {
	ledger := make(MapLedger)

	chain := flow.Mainnet.Chain()

	dal := NewLedgerDAL(ledger, chain)

	address, err := dal.CreateAccount(nil)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_ = dal.GetAccount(address)
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func Test_AccountWithNoKeysCounter(t *testing.T) {
	ledger := make(MapLedger)

	chain := flow.Mainnet.Chain()

	dal := NewLedgerDAL(ledger, chain)

	address, err := dal.CreateAccount(nil)
	require.NoError(t, err)

	countRegister := fullKeyHash(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount)

	ledger.Delete(countRegister)

	require.NotPanics(t, func() {
		_ = dal.GetAccount(address)
	})
}
