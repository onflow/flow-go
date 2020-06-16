package stdmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestIdentiferMap(t *testing.T) {
	idMap, err := NewIdentifierMap(10)

	t.Run("creating new mempool", func(t *testing.T) {
		require.NoError(t, err)
	})

	key1 := unittest.IdentifierFixture()
	id1 := unittest.IdentifierFixture()
	t.Run("appending id to new key", func(t *testing.T) {
		err = idMap.Append(key1, id1)
		require.NoError(t, err)

		// checks the existence of id1 for key
		ids, ok := idMap.Get(key1)
		require.True(t, ok)
		require.Contains(t, ids, id1)
	})

	id2 := unittest.IdentifierFixture()
	t.Run("appending the second id", func(t *testing.T) {
		err = idMap.Append(key1, id2)
		require.NoError(t, err)

		// checks the existence of both id1 and id2 for key1
		ids, ok := idMap.Get(key1)
		require.True(t, ok)
		// both ids should exist
		assert.Contains(t, ids, id1)
		assert.Contains(t, ids, id2)
	})

	// tests against existence of another key, with a shared id (id1)
	key2 := unittest.IdentifierFixture()
	t.Run("appending shared id to another key", func(t *testing.T) {
		err = idMap.Append(key2, id1)
		require.NoError(t, err)

		// checks the existence of both id1 and id2 for key1
		ids, ok := idMap.Get(key1)
		require.True(t, ok)
		// both ids should exist
		assert.Contains(t, ids, id1)
		assert.Contains(t, ids, id2)

		// checks the existence of  id1 for key2
		ids, ok = idMap.Get(key2)
		require.True(t, ok)
		assert.Contains(t, ids, id1)
		assert.NotContains(t, ids, id2)
	})

}
