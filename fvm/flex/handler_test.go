package flex_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/flex"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	flex.RunWithTempLedger(t, func(led atree.Ledger) {
		handler := flex.NewFlexContractHandler(led)

		// test call last executed block without initialization
		b := handler.LastExecutedBlock()
		require.NotNil(t, b)
		require.Equal(t, uint64(0), b.Height)
		require.Equal(t, uint64(0), b.TotalSupply)
		require.Equal(t, uint64(1), b.UUIDIndex)
		require.Equal(t, types.EmptyRootHash, b.StateRoot)
		require.Equal(t, types.EmptyRootHash, b.EventRoot)

		foa := handler.NewFlowOwnedAccount()
		require.NotNil(t, foa)

		// check if uuid index has been auto incremented
		b = handler.LastExecutedBlock()
		require.Equal(t, uint64(2), b.UUIDIndex)

		expectedAddress := models.NewFlexAddress(common.HexToAddress("0x00000000000000000001"))
		require.Equal(t, expectedAddress, foa.Address())

		zeroBalance, err := models.NewBalanceFromAttoFlow(big.NewInt(0))
		require.NoError(t, err)
		require.Equal(t, zeroBalance, foa.Balance())

		balance, err := models.NewBalanceFromAttoFlow(big.NewInt(100))
		require.NoError(t, err)
		vault := models.NewFlowTokenVault(balance)

		foa.Deposit(vault)

		require.NoError(t, err)
		require.Equal(t, balance, foa.Balance())

		v := foa.Withdraw(balance)
		require.NoError(t, err)
		require.Equal(t, balance, v.Balance())

		require.NoError(t, err)
		require.Equal(t, zeroBalance, foa.Balance())
	})
}
