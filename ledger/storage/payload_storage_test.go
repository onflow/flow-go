package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPayloadStorage(t *testing.T) {
	store := unittest.CreateMockPayloadStore()

	n := 2
	updates := make([]ledger.LeafNode, 0, n)
	for i := 0; i < n; i++ {
		path := testutils.PathByUint16LeftPadded(uint16(i))
		payload := testutils.LightPayload(uint16(i), uint16(i))
		updates = append(updates, ledger.LeafNode{
			Hash:    hash.Hash(path),
			Path:    path,
			Payload: *payload,
		})
	}

	err := store.Add(updates)
	require.NoError(t, err)

	for _, update := range updates {
		sPath, sPayload, err := store.Get(update.Hash)
		require.NoError(t, err)
		require.Equal(t, update.Path, sPath)
		require.Equal(t, update.Payload, *sPayload)
	}
}
