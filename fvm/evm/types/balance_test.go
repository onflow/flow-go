package types_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestBalance(t *testing.T) {
	// test attoflow to flow
	bal := types.OneFlowBalance
	require.Equal(t, bal, types.NewBalanceFromUFix64(types.OneFlowInUFix64))

	// 100.0002 Flow
	u, err := cadence.NewUFix64("100.0002")
	require.NoError(t, err)
	require.Equal(t, "100.00020000", u.String())

	bb := types.NewBalanceFromUFix64(u)
	require.Equal(t, "100000200000000000000", types.BalanceToBigInt(bb).String())
	require.False(t, types.BalanceConvertionToUFix64ProneToRoundingError(bb))
	bret, roundedOff, err := types.ConvertBalanceToUFix64(bb)
	require.NoError(t, err)
	require.Equal(t, u, bret)
	require.False(t, roundedOff)

	// rounded off flag
	bal = types.NewBalance(big.NewInt(1))
	require.NoError(t, err)
	require.True(t, types.BalanceConvertionToUFix64ProneToRoundingError(bal))
	bret, roundedOff, err = types.ConvertBalanceToUFix64(bal)
	require.NoError(t, err)
	require.Equal(t, cadence.UFix64(0), bret)
	require.True(t, roundedOff)
}
