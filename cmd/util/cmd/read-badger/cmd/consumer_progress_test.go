package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func TestQuerySingleConsumerProgress(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		progressID := "test_consumer_progress"
		expectedHeight := uint64(12345)

		// Set up a consumer progress entry
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.SetProcessedIndex(rw.Writer(), progressID, expectedHeight)
		})
		require.NoError(t, err)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Query the consumer progress
		err = querySingleConsumerProgress(db, progressID)
		require.NoError(t, err)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, progressID)
		assert.Contains(t, output, "12345")
	})
}

func TestQuerySingleConsumerProgress_NotFound(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		err := querySingleConsumerProgress(db, "nonexistent_consumer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not retrieve processed index")
	})
}

func TestQueryAllConsumerProgress(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		// Set up some consumer progress entries
		testData := map[string]uint64{
			module.ConsumeProgressVerificationBlockHeight:           100,
			module.ConsumeProgressExecutionDataRequesterBlockHeight: 200,
			module.ConsumeProgressExecutionDataIndexerBlockHeight:   300,
			module.ConsumeProgressIngestionEngineBlockHeight:        400,
			module.ConsumeProgressEngineTxErrorMessagesBlockHeight:  500,
			module.ConsumeProgressLastFullBlockHeight:               600,
		}

		for progressID, height := range testData {
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.SetProcessedIndex(rw.Writer(), progressID, height)
			})
			require.NoError(t, err)
		}

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Query all consumer progress
		err := queryAllConsumerProgress(db)
		require.NoError(t, err)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Verify output contains the set entries with their heights
		for progressID, height := range testData {
			assert.Contains(t, output, progressID)
			assert.Contains(t, output, fmt.Sprintf("%d", height))
		}

		// Verify output contains "not found" for entries that were not set
		assert.Contains(t, output, module.ConsumeProgressVerificationChunkIndex)
		assert.Contains(t, output, "not found")
	})
}

func TestQueryAllConsumerProgress_Empty(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Query all consumer progress on empty database
		err := queryAllConsumerProgress(db)
		require.NoError(t, err)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// All entries should show "not found"
		for _, progressID := range allConsumerProgressIDs {
			assert.Contains(t, output, progressID)
		}
		// Count occurrences of "not found" - should be one for each progress ID
		assert.Equal(t, len(allConsumerProgressIDs), bytes.Count([]byte(output), []byte("not found")))
	})
}

func TestSetConsumerProgress_NewEntry(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		progressID := "test_consumer_progress"
		height := uint64(12345)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Set consumer progress with force flag on a new entry
		err := setConsumerProgress(db, progressID, height, true)
		require.NoError(t, err)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, progressID)
		assert.Contains(t, output, "12345")
		assert.NotContains(t, output, "Previous Height")

		// Verify the value was persisted
		var storedHeight uint64
		err = operation.RetrieveProcessedIndex(db.Reader(), progressID, &storedHeight)
		require.NoError(t, err)
		assert.Equal(t, height, storedHeight)
	})
}

func TestSetConsumerProgress_UpdateExisting(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		progressID := "test_consumer_progress"
		initialHeight := uint64(100)
		newHeight := uint64(200)

		// Set up initial consumer progress entry
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.SetProcessedIndex(rw.Writer(), progressID, initialHeight)
		})
		require.NoError(t, err)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Update consumer progress with force flag
		err = setConsumerProgress(db, progressID, newHeight, true)
		require.NoError(t, err)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, progressID)
		assert.Contains(t, output, "Previous Height: 100")
		assert.Contains(t, output, "Set Processed Height: 200")

		// Verify the value was updated
		var storedHeight uint64
		err = operation.RetrieveProcessedIndex(db.Reader(), progressID, &storedHeight)
		require.NoError(t, err)
		assert.Equal(t, newHeight, storedHeight)
	})
}

func TestSetConsumerProgress_WithoutForce_ExistingEntry(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		progressID := "test_consumer_progress"
		initialHeight := uint64(100)
		newHeight := uint64(200)

		// Set up initial consumer progress entry
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.SetProcessedIndex(rw.Writer(), progressID, initialHeight)
		})
		require.NoError(t, err)

		// Attempt to update without force flag
		err = setConsumerProgress(db, progressID, newHeight, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "existing height 100")
		assert.Contains(t, err.Error(), "refusing to set")
		assert.Contains(t, err.Error(), "--force")

		// Verify the original value was not changed
		var storedHeight uint64
		err = operation.RetrieveProcessedIndex(db.Reader(), progressID, &storedHeight)
		require.NoError(t, err)
		assert.Equal(t, initialHeight, storedHeight)
	})
}

func TestSetConsumerProgress_WithoutForce_NewEntry(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		progressID := "test_consumer_progress"
		height := uint64(12345)

		// Attempt to set without force flag on non-existent entry
		err := setConsumerProgress(db, progressID, height, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "refusing to set")
		assert.Contains(t, err.Error(), "--force")

		// Verify no value was written
		var storedHeight uint64
		err = operation.RetrieveProcessedIndex(db.Reader(), progressID, &storedHeight)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}
