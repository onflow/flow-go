package inmem

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/utils/unittest"
)

// should be able to convert
func TestFromSnapshot(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		identities := unittest.IdentityListFixture(10)
		root, result, seal := unittest.BootstrapFixture(identities)
		err := state.Mutate().Bootstrap(root, result, seal)
		require.Nil(t, err)

		t.Run("root snapshot", func(t *testing.T) {
			_, err := FromSnapshot(state.Final())
			require.Nil(t, err)
		})
	})
}
