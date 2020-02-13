package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSnapshot(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		genesis := cluster.Genesis()
		chainID := genesis.ChainID

		state, err := NewState(db, chainID)
		require.Nil(t, err)
		mutator := Mutator{state: state}
		_ = mutator

		// TODO test getting header and collection for each
		t.Run("nonexistent block", func(t *testing.T) {
			t.Skip()
		})

		t.Run("finalized block", func(t *testing.T) {
			t.Skip()
		})

		t.Run("at block ID", func(t *testing.T) {
			t.Skip()
		})
	})
}
