package virtualmachine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

func Test_GenerateUUID(t *testing.T) {

	ledger := make(MapLedger)

	dal := NewLedgerDAL(ledger, flow.Testnet.Chain())
	transactionContext := &TransactionContext{
		ledger: dal,
	}
	require.Equal(t, uint64(0), transactionContext.GenerateUUID())
	require.Equal(t, uint64(1), transactionContext.GenerateUUID())
	require.Equal(t, uint64(2), transactionContext.GenerateUUID())

	dal = NewLedgerDAL(ledger, flow.Testnet.Chain())
	transactionContext = &TransactionContext{
		ledger: dal,
	}
	require.Equal(t, uint64(3), transactionContext.GenerateUUID())
	require.Equal(t, uint64(4), transactionContext.GenerateUUID())
}
