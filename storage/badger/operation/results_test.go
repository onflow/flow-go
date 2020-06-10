// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestResults_InsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.ExecutionResultFixture()

		err := db.Update(InsertExecutionResult(expected))
		require.Nil(t, err)

		var actual flow.ExecutionResult
		err = db.View(RetrieveExecutionResult(expected.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

// Tests mass indexing all ERs by block ID.
func TestResults_MassIndex(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		// insert some results that aren't indexed
		unindexed := make([]*flow.ExecutionResult, 100)
		for i := range unindexed {
			unindexed[i] = unittest.ExecutionResultFixture()
			err := db.Update(InsertExecutionResult(unindexed[i]))
			require.Nil(t, err)

			// confirm these entities aren't indexed
			var resID flow.Identifier
			err = db.View(LookupExecutionResult(unindexed[i].BlockID, &resID))
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		}

		// insert some results that are indexed
		indexed := make([]*flow.ExecutionResult, 100)
		for i := range indexed {
			indexed[i] = unittest.ExecutionResultFixture()
			_ = db.Update(func(tx *badger.Txn) error {
				err := InsertExecutionResult(indexed[i])(tx)
				require.Nil(t, err)
				err = IndexExecutionResult(indexed[i].BlockID, indexed[i].ID())(tx)
				require.Nil(t, err)
				return nil
			})
		}

		// run the mass index migration
		err := IndexExecutionResultsByBlockID(db)
		assert.Nil(t, err)

		// check that all results are indexed
		for _, result := range append(unindexed, indexed...) {
			var resultID flow.Identifier
			err := db.View(LookupExecutionResult(result.BlockID, &resultID))
			assert.Nil(t, err)
			assert.Equal(t, result.ID(), resultID)
		}
	})
}

func BenchmarkResults_MassIndex1000(b *testing.B) {
	benchmarkResults_MassIndex(b, 1000)
}

func BenchmarkResults_MassIndex10000(b *testing.B) {
	benchmarkResults_MassIndex(b, 10000)
}

func BenchmarkResults_MassIndex100000(b *testing.B) {
	benchmarkResults_MassIndex(b, 100000)
}

func benchmarkResults_MassIndex(b *testing.B, n int) {
	unittest.RunWithBadgerDB(b, func(db *badger.DB) {

		setup := func() {
			b.StopTimer()
			err := db.DropPrefix(makePrefix(codeExecutionResult))
			require.Nil(b, err)
			results := make([]*flow.ExecutionResult, n)
			for i := range results {
				results[i] = unittest.ExecutionResultFixture()
				err := db.Update(InsertExecutionResult(results[i]))
				require.Nil(b, err)
			}
			b.StartTimer()
		}

		for i := 0; i < b.N; i++ {
			setup()
			err := IndexExecutionResultsByBlockID(db)
			assert.Nil(b, err)
		}
	})
}
