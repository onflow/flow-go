package state_test

import (
	"testing"

	"github.com/onflow/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestAccountEncoding(t *testing.T) {
	acc := state.NewAccount(
		testutils.RandomCommonAddress(t),
		testutils.RandomBigInt(1000),
		uint64(2),
		common.BytesToHash([]byte{1}),
		[]byte{2},
	)

	encoded, err := acc.Encode()
	require.NoError(t, err)

	ret, err := state.DecodeAccount(encoded)
	require.NoError(t, err)
	require.Equal(t, acc, ret)
}
