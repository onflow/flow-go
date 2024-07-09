package pebble_test

import (
	"reflect"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/testutil"
	bstorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestUpsertAndRetrieveComputationResult(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := testutil.ComputationResultFixture(t)
		crStorage := bstorage.NewComputationResultUploadStatus(db)
		crId := expected.ExecutableBlock.ID()

		// True case - upsert
		testUploadStatus := true
		err := crStorage.Upsert(crId, testUploadStatus)
		require.NoError(t, err)

		actualUploadStatus, err := crStorage.ByID(crId)
		require.NoError(t, err)

		assert.Equal(t, testUploadStatus, actualUploadStatus)

		// False case - update
		testUploadStatus = false
		err = crStorage.Upsert(crId, testUploadStatus)
		require.NoError(t, err)

		actualUploadStatus, err = crStorage.ByID(crId)
		require.NoError(t, err)

		assert.Equal(t, testUploadStatus, actualUploadStatus)
	})
}

func TestRemoveComputationResults(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		t.Run("Remove ComputationResult", func(t *testing.T) {
			expected := testutil.ComputationResultFixture(t)
			crId := expected.ExecutableBlock.ID()
			crStorage := bstorage.NewComputationResultUploadStatus(db)

			testUploadStatus := true
			err := crStorage.Upsert(crId, testUploadStatus)
			require.NoError(t, err)

			_, err = crStorage.ByID(crId)
			require.NoError(t, err)

			err = crStorage.Remove(crId)
			require.NoError(t, err)

			_, err = crStorage.ByID(crId)
			assert.Error(t, err)
		})
	})
}

func TestListComputationResults(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		t.Run("List all ComputationResult with given status", func(t *testing.T) {
			expected := [...]*execution.ComputationResult{
				testutil.ComputationResultFixture(t),
				testutil.ComputationResultFixture(t),
			}
			crStorage := bstorage.NewComputationResultUploadStatus(db)

			// Store a list of ComputationResult instances first
			expectedIDs := make(map[string]bool, 0)
			for _, cr := range expected {
				crId := cr.ExecutableBlock.ID()
				expectedIDs[crId.String()] = true
				err := crStorage.Upsert(crId, true)
				require.NoError(t, err)
			}
			// Add in entries with non-targeted status
			unexpected := [...]*execution.ComputationResult{
				testutil.ComputationResultFixture(t),
				testutil.ComputationResultFixture(t),
			}
			for _, cr := range unexpected {
				crId := cr.ExecutableBlock.ID()
				err := crStorage.Upsert(crId, false)
				require.NoError(t, err)
			}

			// Get the list of IDs for stored instances
			crIDs, err := crStorage.GetIDsByUploadStatus(true)
			require.NoError(t, err)

			crIDsStrMap := make(map[string]bool, 0)
			for _, crID := range crIDs {
				crIDsStrMap[crID.String()] = true
			}

			assert.True(t, reflect.DeepEqual(crIDsStrMap, expectedIDs))
		})
	})
}
