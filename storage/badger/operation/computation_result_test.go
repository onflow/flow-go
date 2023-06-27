package operation

import (
	"reflect"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertAndUpdateAndRetrieveComputationResultUpdateStatus(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.ID()

		t.Run("Update existing ComputationResult", func(t *testing.T) {
			// insert as False
			testUploadStatusVal := false

			err := db.Update(InsertComputationResultUploadStatus(expectedId, testUploadStatusVal))
			require.NoError(t, err)

			var actualUploadStatus bool
			err = db.View(GetComputationResultUploadStatus(expectedId, &actualUploadStatus))
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			// update to True
			testUploadStatusVal = true
			err = db.Update(UpdateComputationResultUploadStatus(expectedId, testUploadStatusVal))
			require.NoError(t, err)

			// check if value is updated
			err = db.View(GetComputationResultUploadStatus(expectedId, &actualUploadStatus))
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)
		})

		t.Run("Update non-existed ComputationResult", func(t *testing.T) {
			testUploadStatusVal := true
			randomFlowID := flow.Identifier{}
			err := db.Update(UpdateComputationResultUploadStatus(randomFlowID, testUploadStatusVal))
			require.Error(t, err)
			require.Equal(t, err, storage.ErrNotFound)
		})
	})
}

func TestUpsertAndRetrieveComputationResultUpdateStatus(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.ID()

		t.Run("Upsert ComputationResult", func(t *testing.T) {
			// first upsert as false
			testUploadStatusVal := false

			err := db.Update(UpsertComputationResultUploadStatus(expectedId, testUploadStatusVal))
			require.NoError(t, err)

			var actualUploadStatus bool
			err = db.View(GetComputationResultUploadStatus(expectedId, &actualUploadStatus))
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			// upsert to true
			testUploadStatusVal = true
			err = db.Update(UpsertComputationResultUploadStatus(expectedId, testUploadStatusVal))
			require.NoError(t, err)

			// check if value is updated
			err = db.View(GetComputationResultUploadStatus(expectedId, &actualUploadStatus))
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)
		})
	})
}

func TestRemoveComputationResultUploadStatus(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := testutil.ComputationResultFixture(t)
		expectedId := expected.ExecutableBlock.ID()

		t.Run("Remove ComputationResult", func(t *testing.T) {
			testUploadStatusVal := true

			err := db.Update(InsertComputationResultUploadStatus(expectedId, testUploadStatusVal))
			require.NoError(t, err)

			var actualUploadStatus bool
			err = db.View(GetComputationResultUploadStatus(expectedId, &actualUploadStatus))
			require.NoError(t, err)

			assert.Equal(t, testUploadStatusVal, actualUploadStatus)

			err = db.Update(RemoveComputationResultUploadStatus(expectedId))
			require.NoError(t, err)

			err = db.View(GetComputationResultUploadStatus(expectedId, &actualUploadStatus))
			assert.NotNil(t, err)
		})
	})
}

func TestListComputationResults(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
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
				err := db.Update(InsertComputationResultUploadStatus(expectedId, true))
				require.NoError(t, err)
			}

			// Get the list of IDs of stored ComputationResult
			crIDs := make([]flow.Identifier, 0)
			err := db.View(GetBlockIDsByStatus(&crIDs, true))
			require.NoError(t, err)
			crIDsStrMap := make(map[string]bool, 0)
			for _, crID := range crIDs {
				crIDsStrMap[crID.String()] = true
			}

			assert.True(t, reflect.DeepEqual(crIDsStrMap, expectedIDs))
		})
	})
}
