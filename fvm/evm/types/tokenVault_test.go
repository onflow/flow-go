package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestVault(t *testing.T) {

	vault1 := types.NewFlowTokenVault(types.MakeABalanceInFlow(3))

	vault2, err := vault1.Withdraw(types.OneFlowBalance)
	require.NoError(t, err)

	require.Equal(t, types.MakeABalanceInFlow(2), vault1.Balance())
	require.Equal(t, types.OneFlowBalance, vault2.Balance())

	toBeDeposited := types.NewFlowTokenVault(types.OneFlowBalance)
	err = vault1.Deposit(toBeDeposited)
	require.NoError(t, err)
	require.Equal(t, types.MakeABalanceInFlow(3), vault1.Balance())
	require.Equal(t, types.EmptyBalance, toBeDeposited.Balance())
}
