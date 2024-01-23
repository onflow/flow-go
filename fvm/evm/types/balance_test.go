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
	bal := types.NewBalanceFromAttoFlow(types.OneFlowInAttoFlow)
	require.Equal(t, bal, types.NewBalance(types.OneFlow))

	conv := bal.ToAttoFlow()
	require.Equal(t, types.OneFlowInAttoFlow, conv)

	// encoding decoding
	encoded, err := bal.Encode()
	require.NoError(t, err)
	ret, err := types.DecodeBalance(encoded)
	require.NoError(t, err)
	require.Equal(t, bal, ret)

	// 100.0002 Flow
	u, err := cadence.NewUFix64("100.0002")
	require.NoError(t, err)
	require.Equal(t, "100.00020000", u.String())

	bb := types.NewBalance(u)
	require.Equal(t, "100000200000000000000", bb.ToAttoFlow().String())
	require.False(t, bb.HasUFix64RoundingError())
	bret, err := bb.ToUFix64()
	require.NoError(t, err)
	require.Equal(t, u, bret)

	// rounded off flag
	bal = types.NewBalanceFromAttoFlow(big.NewInt(1))
	require.NoError(t, err)
	require.True(t, bal.HasUFix64RoundingError())
	bret, err = bal.ToUFix64()
	require.NoError(t, err)
	require.Equal(t, cadence.UFix64(0), bret)
}
