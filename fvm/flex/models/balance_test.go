package models_test

import (
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/stretchr/testify/require"
)

var oneAttoFlow = big.NewInt(1)

// var thousandsAttoFlow = new(big.Int).Mul(oneAttoFlow, big.NewInt(1000))
var oneFlow = new(big.Int).Mul(oneAttoFlow, big.NewInt(1e18))

func TestHandler(t *testing.T) {
	// test attoflow to flow

	bal, err := models.NewBalanceFromAttoFlow(oneFlow)
	require.NoError(t, err)

	conv := bal.ToAttoFlow()
	require.Equal(t, oneFlow, conv)

	// 100.0002 Flow
	u, err := cadence.NewUFix64("100.0002")
	require.NoError(t, err)
	require.Equal(t, "100.00020000", u.String())

	bb := models.Balance(u).ToAttoFlow()
	require.Equal(t, "100000200000000000000", bb.String())
}

// TODO add test for encoding decoding
