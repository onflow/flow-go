package state_test

import (
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestCollection(t *testing.T) {

	cp := setupTestCollection(t)
	c1, err := cp.NewCollection()
	require.NoError(t, err)

	key1 := []byte("A")
	key2 := []byte("B")
	value1 := []byte{1}
	value2 := []byte{2}

	// get value for A
	ret, err := c1.Get(key1)
	require.NoError(t, err)
	require.Empty(t, ret)

	// set value1 for A
	err = c1.Set(key1, value1)
	require.NoError(t, err)

	ret, err = c1.Get(key1)
	require.NoError(t, err)
	require.Equal(t, ret, value1)

	err = c1.Remove(key1)
	require.NoError(t, err)

	ret, err = c1.Get(key1)
	require.NoError(t, err)
	require.Empty(t, ret)

	err = c1.Set(key2, value2)
	require.NoError(t, err)

	c2, err := cp.CollectionByID(c1.CollectionID())
	require.NoError(t, err)

	ret, err = c2.Get(key2)
	require.NoError(t, err)
	require.Equal(t, value2, ret)

	// destroy
	keys, err := c1.Destroy()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, key2, keys[0])

	_, err = cp.CollectionByID(c1.CollectionID())
	require.Error(t, err)
}

func setupTestCollection(t *testing.T) *state.CollectionProvider {
	ledger := testutils.GetSimpleValueStore()
	cp, err := state.NewCollectionProvider(atree.Address{1, 2, 3, 4, 5, 6, 7, 8}, ledger)
	require.NoError(t, err)
	return cp
}
