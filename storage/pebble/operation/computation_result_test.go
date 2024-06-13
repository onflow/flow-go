package operation_test

import (
	"reflect"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertAndUpdateAndRetrieveComputationResultUpdateStatus(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.ID()

		t.Run("Update existing ComputationResult", func(t *testing.T) {
			// insert as False
			testUploadStatusVal := false

			err := operation.InsertComputationResultUploadStatus(expectedId, testUploadStatusVal)(db)
			require.NoError(t, err)

			var actualUploadStatus bool
			err = operation.GetComputationResultUploadStatus(expectedId, &actualUploadStatus)(db)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			// update to True
			testUploadStatusVal = true
			err = operation.UpdateComputationResultUploadStatus(expectedId, testUploadStatusVal)(db)
			require.NoError(t, err)

			// check if value is updated
			err = operation.GetComputationResultUploadStatus(expectedId, &actualUploadStatus)(db)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)
		})

		t.Run("Update non-existed ComputationResult", func(t *testing.T) {
			testUploadStatusVal := true
			randomFlowID := flow.Identifier{}
			err := operation.UpdateComputationResultUploadStatus(randomFlowID, testUploadStatusVal)(db)
			require.Error(t, err)
			require.Equal(t, err, storage.ErrNotFound)
		})
	})
}

func TestUpsertAndRetrieveComputationResultUpdateStatus(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.ID()

		t.Run("Upsert ComputationResult", func(t *testing.T) {
			// first upsert as false
			testUploadStatusVal := false

			err := operation.UpsertComputationResultUploadStatus(expectedId, testUploadStatusVal)(db)
			require.NoError(t, err)

			var actualUploadStatus bool
			err = operation.GetComputationResultUploadStatus(expectedId, &actualUploadStatus)(db)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			// upsert to true
			testUploadStatusVal = true
			err = operation.UpsertComputationResultUploadStatus(expectedId, testUploadStatusVal)(db)
			require.NoError(t, err)

			// check if value is updated
			err = operation.GetComputationResultUploadStatus(expectedId, &actualUploadStatus)(db)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)
		})
	})
}

func TestRemoveComputationResultUploadStatus(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.ID()

		t.Run("Remove ComputationResult", func(t *testing.T) {
			testUploadStatusVal := true

			err := operation.InsertComputationResultUploadStatus(expectedId, testUploadStatusVal)(db)
			require.NoError(t, err)

			var actualUploadStatus bool
			err = operation.GetComputationResultUploadStatus(expectedId, &actualUploadStatus)(db)
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			err = operation.RemoveComputationResultUploadStatus(expectedId)(db)
			require.NoError(t, err)

			err = operation.GetComputationResultUploadStatus(expectedId, &actualUploadStatus)(db)
			assert.NotNil(t, err)
		})
	})
}

func TestListComputationResults(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := [...]*execution.ComputationResult{
			testutil.ComputationResultFixture(t),
			testutil.ComputationResultFixture(t),
		}
		t.Run("List all ComputationResult with status True", func(t *testing.T) {
			expectedIDs := make(map[string]bool, 0)
			// Store a list of ComputationResult instances first
			for _, cr := range expected {
				expectedId := cr.ExecutableBlock.ID()
				expectedIDs[expectedId.String()] = true
				err := operation.InsertComputationResultUploadStatus(expectedId, true)(db)
				require.NoError(t, err)
			}

			// Get the list of IDs of stored ComputationResult
			crIDs := make([]flow.Identifier, 0)
			err := operation.GetBlockIDsByStatus(&crIDs, true)(db)
			require.NoError(t, err)
			crIDsStrMap := make(map[string]bool, 0)
			for _, crID := range crIDs {
				crIDsStrMap[crID.String()] = true
			}

			assert.True(t, reflect.DeepEqual(crIDsStrMap, expectedIDs))
		})
	})
}
