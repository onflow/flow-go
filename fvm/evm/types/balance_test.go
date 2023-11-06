package types_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var oneAttoFlow = big.NewInt(1)
var oneFlow = new(big.Int).Mul(oneAttoFlow, big.NewInt(1e18))

func TestBalance(t *testing.T) {
	// test attoflow to flow

	bal, err := types.NewBalanceFromAttoFlow(oneFlow)
	require.NoError(t, err)

	conv := bal.ToAttoFlow()
	require.Equal(t, oneFlow, conv)

	// encoding decoding
	ret, err := types.DecodeBalance(bal.Encode())
	require.NoError(t, err)
	require.Equal(t, bal, ret)

	// 100.0002 Flow
	u, err := cadence.NewUFix64("100.0002")
	require.NoError(t, err)
	require.Equal(t, "100.00020000", u.String())

	bb := types.Balance(u).ToAttoFlow()
	require.Equal(t, "100000200000000000000", bb.String())

	// invalid conversion
	_, err = types.NewBalanceFromAttoFlow(big.NewInt(1))
	require.Error(t, err)

}
