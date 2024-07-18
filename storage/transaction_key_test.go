package storage_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txID := unittest.IdentifierFixture()
	key := storage.KeyFromBlockIDTransactionID(blockID, txID)
	bID, tID, err := storage.KeyToBlockIDTransactionID(key)
	require.NoError(t, err)
	require.Equal(t, blockID, bID)
	require.Equal(t, txID, tID)
}

func TestIndexKeyConversion(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	txIndex := rand.Uint32()
	key := storage.KeyFromBlockIDIndex(blockID, txIndex)
	bID, tID, err := storage.KeyToBlockIDIndex(key)
	require.NoError(t, err)
	require.Equal(t, blockID, bID)
	require.Equal(t, txIndex, tID)
}
