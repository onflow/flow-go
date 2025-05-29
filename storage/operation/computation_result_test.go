package operation_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func TestUpsertAndRetrieveComputationResultUpdateStatus(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.BlockID()

		t.Run("Upsert ComputationResult", func(t *testing.T) {
			// first upsert as false
			testUploadStatusVal := false

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertComputationResultUploadStatus(rw.Writer(), expectedId, testUploadStatusVal)
			})
			require.NoError(t, err)

			var actualUploadStatus bool
			err = operation.GetComputationResultUploadStatus(db.Reader(), expectedId, &actualUploadStatus)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			// upsert to true
			testUploadStatusVal = true
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertComputationResultUploadStatus(rw.Writer(), expectedId, testUploadStatusVal)
			})
			require.NoError(t, err)

			// check if value is updated
			err = operation.GetComputationResultUploadStatus(db.Reader(), expectedId, &actualUploadStatus)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)
		})
	})
}

func TestRemoveComputationResultUploadStatus(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.BlockID()

		t.Run("Remove ComputationResult", func(t *testing.T) {
			testUploadStatusVal := true

			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertComputationResultUploadStatus(rw.Writer(), expectedId, testUploadStatusVal)
			})
			require.NoError(t, err)

			var actualUploadStatus bool
			err = operation.GetComputationResultUploadStatus(db.Reader(), expectedId, &actualUploadStatus)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.RemoveComputationResultUploadStatus(rw.Writer(), expectedId)
			})
			require.NoError(t, err)

			err = operation.GetComputationResultUploadStatus(db.Reader(), expectedId, &actualUploadStatus)
			assert.NotNil(t, err)
		})
	})
}

func TestListComputationResults(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := [...]*execution.ComputationResult{
			testutil.ComputationResultFixture(t),
			testutil.ComputationResultFixture(t),
		}
		t.Run("List all ComputationResult with status True", func(t *testing.T) {
			expectedIDs := make(map[string]bool, 0)
			// Store a list of ComputationResult instances first
			for _, cr := range expected {
				expectedId := cr.ExecutableBlock.BlockID()
				expectedIDs[expectedId.String()] = true
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.UpsertComputationResultUploadStatus(rw.Writer(), expectedId, true)
				})
				require.NoError(t, err)
			}

			// Get the list of IDs of stored ComputationResult
			crIDs := make([]flow.Identifier, 0)
			err := operation.GetBlockIDsByStatus(db.Reader(), &crIDs, true)
			require.NoError(t, err)
			crIDsStrMap := make(map[string]bool, 0)
			for _, crID := range crIDs {
				crIDsStrMap[crID.String()] = true
			}

			assert.True(t, reflect.DeepEqual(crIDsStrMap, expectedIDs))
		})
	})
}
