// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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

func TestResults_IndexByServiceEvents(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		result1 := unittest.ExecutionResultFixture()
		result2 := unittest.ExecutionResultFixture()
		result3 := unittest.ExecutionResultFixture()
		height1 := uint64(21)
		height2 := uint64(37)
		height3 := uint64(55)
		eventType := flow.ServiceEventCommit

		// inserting 3 results at different height each has a ServiceEventCommit
		err := db.Update(IndexExecutionResultByServiceEventTypeAndHeight(result1.ID(), eventType, height1))
		require.NoError(t, err)

		err = db.Update(IndexExecutionResultByServiceEventTypeAndHeight(result2.ID(), eventType, height2))
		require.NoError(t, err)

		err = db.Update(IndexExecutionResultByServiceEventTypeAndHeight(result3.ID(), eventType, height3))
		require.NoError(t, err)

		// insert result 2 again to make sure we tolerate duplicates
		// it is possible for two or more events of the same type to be from the same height
		err = db.Update(IndexExecutionResultByServiceEventTypeAndHeight(result2.ID(), eventType, height2))
		require.NoError(t, err)

		t.Run("retrieve exact height match", func(t *testing.T) {
			//t.Parallel()
			var actualResult flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEventType(height1, eventType, &actualResult))
			require.NoError(t, err)
			require.Equal(t, result1.ID(), actualResult)

			err = db.View(LookupLastExecutionResultForServiceEventType(height2, eventType, &actualResult))
			require.NoError(t, err)
			require.Equal(t, result2.ID(), actualResult)

			err = db.View(LookupLastExecutionResultForServiceEventType(height3, eventType, &actualResult))
			require.NoError(t, err)
			require.Equal(t, result3.ID(), actualResult)
		})

		t.Run("different event type retrieve nothing", func(t *testing.T) {
			//t.Parallel()
			var actualResult flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEventType(height1, flow.ServiceEventSetup, &actualResult))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("finds highest but not higher than given", func(t *testing.T) {
			//t.Parallel()
			var actualResult flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEventType(height3-1, eventType, &actualResult))
			require.NoError(t, err)
			require.Equal(t, result2.ID(), actualResult)
		})

		t.Run("finds highest", func(t *testing.T) {
			//t.Parallel()
			var actualResult flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEventType(height3+1, eventType, &actualResult))
			require.NoError(t, err)
			require.Equal(t, result3.ID(), actualResult)
		})

		t.Run("height below lowest entry returns nothing", func(t *testing.T) {
			//t.Parallel()
			var actualResult flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEventType(height1-1, flow.ServiceEventSetup, &actualResult))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

	})
}
