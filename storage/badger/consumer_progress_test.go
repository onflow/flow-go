package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func TestConsumerProgressInitializer(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		t.Run("Initialize with default index", func(t *testing.T) {
			cpi := NewConsumerProgress(db, "test_consumer1")
			progress, err := cpi.Initialize(100)
			require.NoError(t, err)

			index, err := progress.ProcessedIndex()
			require.NoError(t, err)
			assert.Equal(t, uint64(100), index)
		})

		t.Run("Initialize when already initialized", func(t *testing.T) {
			cpi := NewConsumerProgress(db, "test_consumer2")

			// First initialization
			_, err := cpi.Initialize(100)
			require.NoError(t, err)

			// Second initialization with different index
			progress, err := cpi.Initialize(200)
			require.NoError(t, err)

			// Should still return the original index
			index, err := progress.ProcessedIndex()
			require.NoError(t, err)
			assert.Equal(t, uint64(100), index)
		})

		t.Run("SetProcessedIndex and ProcessedIndex", func(t *testing.T) {
			cpi := NewConsumerProgress(db, "test_consumer3")
			progress, err := cpi.Initialize(100)
			require.NoError(t, err)

			err = progress.SetProcessedIndex(150)
			require.NoError(t, err)

			index, err := progress.ProcessedIndex()
			require.NoError(t, err)
			assert.Equal(t, uint64(150), index)
		})
	})
}

func TestConsumerProgressBatchSet(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		t.Run("BatchSetProcessedIndex and ProcessedIndex", func(t *testing.T) {
			cpi := NewConsumerProgress(db, "test_consumer")
			progress, err := cpi.Initialize(100)
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(r storage.ReaderBatchWriter) error {
				err := progress.BatchSetProcessedIndex(150, r)
				require.NoError(t, err)

				// Verify the index is not set until batch is committed
				index, err := progress.ProcessedIndex()
				require.NoError(t, err)
				assert.Equal(t, uint64(100), index)

				return nil
			})
			require.NoError(t, err)

			// Verify the index was updated after batch commit
			index, err := progress.ProcessedIndex()
			require.NoError(t, err)
			assert.Equal(t, uint64(150), index)
		})
	})
}
