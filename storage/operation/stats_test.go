package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSummarizeKeysByFirstByteConcurrent(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			// insert random events
			b := unittest.IdentifierFixture()
			events := unittest.EventsFixture(30)
			for _, evt := range events {
				err := operation.InsertEvent(rw.Writer(), b, evt)
				if err != nil {
					return err
				}
			}

			// insert 100 chunk data packs
			for i := 0; i < 100; i++ {
				collectionID := unittest.IdentifierFixture()
				cdp := &storage.StoredChunkDataPack{
					ChunkID:      unittest.IdentifierFixture(),
					StartState:   unittest.StateCommitmentFixture(),
					Proof:        []byte{'p'},
					CollectionID: collectionID,
				}
				err := operation.InsertChunkDataPack(rw.Writer(), cdp)
				if err != nil {
					return err
				}
			}

			// insert 20 results
			for i := 0; i < 20; i++ {
				result := unittest.ExecutionResultFixture()
				err := operation.InsertExecutionResult(rw.Writer(), result)
				if err != nil {
					return err
				}
			}

			return nil
		})
		require.NoError(t, err)

		// summarize keys by first byte
		stats, err := operation.SummarizeKeysByFirstByteConcurrent(unittest.Logger(), db.Reader(), 10)
		require.NoError(t, err)

		// print
		operation.PrintStats(unittest.Logger(), stats)

		for i := 0; i < 256; i++ {
			count := 0
			if i == 102 { // events
				count = 30
			} else if i == 100 { // CDP
				count = 100
			} else if i == 36 { // results
				count = 20
			}
			require.Equal(t, count, stats[byte(i)].Count, "byte %d", i)
		}
	})
}
