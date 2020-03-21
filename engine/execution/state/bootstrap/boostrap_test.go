package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisStateCommitment(t *testing.T) {
	unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {

		ls, err := ledger.NewTrieStorage(db)
		require.NoError(t, err)

		//emptyStateCommitment := ls.LatestStateCommitment()

		newStateCommitment, err := BootstrapLedger(ls)
		require.NoError(t, err)

		assert.Equal(t, newStateCommitment, flow.GenesisStateCommitment)
	})
}
