package storage_test

import (
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func TestEphemeralStorage(t *testing.T) {

	parent := testutils.GetSimpleValueStore()
	// preset value
	owner := []byte("owner")
	key1 := []byte("key1")
	value1 := []byte{1}
	value2 := []byte{2}
	err := parent.SetValue(owner, key1, value1)
	require.NoError(t, err)

	s := storage.NewEphemeralStorage(parent)
	ret, err := s.GetValue(owner, key1)
	require.NoError(t, err)
	require.Equal(t, value1, ret)
	found, err := s.ValueExists(owner, key1)
	require.NoError(t, err)
	require.True(t, found)

	// test set value
	err = s.SetValue(owner, key1, value2)
	require.NoError(t, err)
	ret, err = s.GetValue(owner, key1)
	require.NoError(t, err)
	require.Equal(t, value2, ret)
	// the parent should still return the value1
	ret, err = parent.GetValue(owner, key1)
	require.NoError(t, err)
	require.Equal(t, value1, ret)

	// test allocate slab id
	_, err = s.AllocateSlabIndex(owner)
	require.Error(t, err)

	// setup account
	err = s.SetValue(owner, []byte(flow.AccountStatusKey), environment.NewAccountStatus().ToBytes())
	require.NoError(t, err)

	sid, err := s.AllocateSlabIndex(owner)
	require.NoError(t, err)
	expected := atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 1})
	require.Equal(t, expected, sid)

	sid, err = s.AllocateSlabIndex(owner)
	require.NoError(t, err)
	expected = atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 2})
	require.Equal(t, expected, sid)

	// fetch delta
	delta := s.StorageRegisterUpdates()
	require.Len(t, delta, 2)
	ret = delta[flow.RegisterID{
		Owner: string(owner),
		Key:   string(key1),
	}]
	require.Equal(t, value2, ret)
}
