package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertExecuted(t *testing.T) {
	chain, _, _ := unittest.ChainFixture(6)
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("setup and bootstrap", func(t *testing.T) {
			for _, block := range chain {
				require.NoError(t, db.Update(operation.InsertHeader(block.Header.ID(), block.Header)))
			}

			root := chain[0].Header
			require.NoError(t,
				db.Update(operation.InsertExecutedBlock(root.ID())),
			)

			var height uint64
			var blockID flow.Identifier
			require.NoError(t,
				db.View(GetHighestExecutedBlock(&height, &blockID)),
			)

			require.Equal(t, root.ID(), blockID)
			require.Equal(t, root.Height, height)
		})

		t.Run("insert and get", func(t *testing.T) {
			header1 := chain[1].Header
			require.NoError(t,
				db.Update(UpdateHighestExecutedBlockIfHigher(header1)),
			)

			var height uint64
			var blockID flow.Identifier
			require.NoError(t,
				db.View(GetHighestExecutedBlock(&height, &blockID)),
			)

			require.Equal(t, header1.ID(), blockID)
			require.Equal(t, header1.Height, height)
		})

		t.Run("insert more and get highest", func(t *testing.T) {
			header2 := chain[2].Header
			header3 := chain[3].Header
			require.NoError(t,
				db.Update(UpdateHighestExecutedBlockIfHigher(header2)),
			)
			require.NoError(t,
				db.Update(UpdateHighestExecutedBlockIfHigher(header3)),
			)
			var height uint64
			var blockID flow.Identifier
			require.NoError(t,
				db.View(GetHighestExecutedBlock(&height, &blockID)),
			)

			require.Equal(t, header3.ID(), blockID)
			require.Equal(t, header3.Height, height)
		})

		t.Run("insert lower height later and get highest", func(t *testing.T) {
			header5 := chain[5].Header
			header4 := chain[4].Header
			require.NoError(t,
				db.Update(UpdateHighestExecutedBlockIfHigher(header5)),
			)
			require.NoError(t,
				db.Update(UpdateHighestExecutedBlockIfHigher(header4)),
			)
			var height uint64
			var blockID flow.Identifier
			require.NoError(t,
				db.View(GetHighestExecutedBlock(&height, &blockID)),
			)

			require.Equal(t, header5.ID(), blockID)
			require.Equal(t, header5.Height, height)
		})
	})
}
