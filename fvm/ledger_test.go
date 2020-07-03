package fvm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

func TestLedgerDAL_UUID(t *testing.T) {

	ledger := make(MapLedger)

	dal := NewLedgerDAL(ledger, flow.Testnet.Chain())

	uuid, err := dal.GetUUID() // start from zero
	require.NoError(t, err)

	require.Equal(t, uint64(0), uuid)
	dal.SetUUID(5)

	//create new ledgerDAL
	dal = NewLedgerDAL(ledger, flow.Testnet.Chain())
	uuid, err = dal.GetUUID() // should read saved value
	require.NoError(t, err)

	require.Equal(t, uint64(5), uuid)
}
