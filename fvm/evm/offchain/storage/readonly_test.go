package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestReadOnlyStorage(t *testing.T) {
	parent := testutils.GetSimpleValueStore()

	owner := []byte("owner")
	key1 := []byte("key1")
	value1 := []byte{1}
	err := parent.SetValue(owner, key1, value1)
	require.NoError(t, err)

	rs := storage.NewReadOnlyStorage(parent)
	ret, err := rs.GetValue(owner, key1)
	require.NoError(t, err)
	require.Equal(t, value1, ret)

	found, err := rs.ValueExists(owner, key1)
	require.NoError(t, err)
	require.True(t, found)

	err = rs.SetValue(owner, key1, value1)
	require.NoError(t, err)

	_, err = rs.AllocateSlabIndex(owner)
	require.Error(t, err)

}
