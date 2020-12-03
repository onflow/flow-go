package inmem

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/utils/unittest"
)

// should be able to convert
func TestFromSnapshot(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		identities := unittest.IdentityListFixture(10, unittest.WithAllRoles())
		root, result, seal := unittest.BootstrapFixture(identities)
		err := state.Mutate().Bootstrap(root, result, seal)
		require.Nil(t, err)

		t.Run("root snapshot", func(t *testing.T) {
			// TODO need root QC stored for this test to pass
			t.Skip()
			_, err = FromSnapshot(state.Final())
			require.Nil(t, err)
		})

		t.Run("non-root snapshot", func(t *testing.T) {
			// add a block on the root block and a valid child
			block1 := unittest.BlockWithParentFixture(root.Header)
			block1.SetPayload(flow.EmptyPayload())
			err = state.Mutate().Extend(&block1)
			require.Nil(t, err)
			err = state.Mutate().MarkValid(block1.ID())
			require.Nil(t, err)
			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(flow.EmptyPayload())
			err = state.Mutate().Extend(&block2)
			require.Nil(t, err)
			err = state.Mutate().MarkValid(block2.ID())
			require.Nil(t, err)

			_, err = FromSnapshot(state.AtBlockID(block1.ID()))
			require.Nil(t, err)
			// TODO compare serialization
		})

		// TODO
		t.Run("epoch 1", func(t *testing.T) {
			t.Run("staking phase", func(t *testing.T) {})
			t.Run("setup phase", func(t *testing.T) {})
			t.Run("committed phase", func(t *testing.T) {})
		})

		// TODO
		t.Run("epoch 2", func(t *testing.T) {
			t.Run("staking phase", func(t *testing.T) {})
			t.Run("setup phase", func(t *testing.T) {})
			t.Run("committed phase", func(t *testing.T) {})
		})
	})
}
