package sync_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/sync"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestBlockMetaEncodingDecoding(t *testing.T) {
	bm := sync.NewBlockMeta(1, 2, testutils.RandomCommonHash(t))

	ret, err := sync.BlockMetaFromEncoded(bm.Encode())
	require.NoError(t, err)
	require.Equal(t, ret, bm)
}
