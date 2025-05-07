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
		const testConsumer = "test_consumer"

		t.Run("Initialize with default index", func(t *testing.T) {
			cpi := NewConsumerProgress(db, testConsumer)
			progress, err := cpi.Initialize(100)
			require.NoError(t, err)

			index, err := progress.ProcessedIndex()
			require.NoError(t, err)
			assert.Equal(t, uint64(100), index)
		})

		t.Run("Initialize when already initialized", func(t *testing.T) {
			cpi := NewConsumerProgress(db, testConsumer)

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
			cpi := NewConsumerProgress(db, testConsumer)
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
