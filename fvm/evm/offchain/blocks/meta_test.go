package blocks_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestBlockMetaEncodingDecoding(t *testing.T) {
	bm := blocks.NewMeta(1, 2, testutils.RandomCommonHash(t))

	ret, err := blocks.MetaFromEncoded(bm.Encode())
	require.NoError(t, err)
	require.Equal(t, ret, bm)
}
