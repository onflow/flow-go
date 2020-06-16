package stdmap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestIdentiferMap(t *testing.T) {
	idMap, err := NewIdentifierMap(10)

	t.Run("no error on creating a new mempool", func(t *testing.T) {
		require.NoError(t, err)
	})

	key := unittest.IdentifierFixture()
	id1 := unittest.IdentifierFixture()
	t.Run("appending an id to a new key", func(t *testing.T) {
		err = idMap.Append(key, id1)
		require.NoError(t, err)

		// checks the existence of id1 for key
		ids, ok := idMap.Get(key)
		require.True(t, ok)
		require.Contains(t, ids, id1)
	})
}
