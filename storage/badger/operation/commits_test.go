package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestStateCommitments(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.StateCommitmentFixture()
		id := unittest.IdentifierFixture()
		err := db.Update(InsertCommit(id, expected))
		require.Nil(t, err)

		var actual flow.StateCommitment
		err = db.View(RetrieveCommit(id, &actual))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}

func TestStateCommitmentsIndexAndLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		comm := unittest.StateCommitmentFixture()
		blockId := unittest.IdentifierFixture()
		finalId := unittest.IdentifierFixture()

		err := db.Update(func(tx *badger.Txn) error {
			if err := InsertCommit(blockId, comm)(tx); err != nil {
				return err
			}
			if err := IndexCommit(finalId, comm)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		var actual flow.StateCommitment
		err = db.View(RetrieveCommit(blockId, &actual))
		require.Nil(t, err)
		assert.Equal(t, comm, actual)

		var actualFinal flow.StateCommitment
		err = db.View(LookupCommit(finalId, &actualFinal))
		require.Nil(t, err)
		assert.Equal(t, comm, actualFinal)
	})
}
