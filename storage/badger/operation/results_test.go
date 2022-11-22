// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

		t.Run("inserting", func(t *testing.T) {

			err := db.Update(IndexByServiceEvent(height1, result1.ID(), eventType))
			require.NoError(t, err)

			err = db.Update(IndexByServiceEvent(height2, result2.ID(), eventType))
			require.NoError(t, err)

			err = db.Update(IndexByServiceEvent(height3, result3.ID(), eventType))
			require.NoError(t, err)
		})

		t.Run("retrieve exact height match", func(t *testing.T) {
			var actualResults flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEvents(height1, eventType, &actualResults))
			require.NoError(t, err)
			require.Equal(t, result1.ID(), actualResults)

			err = db.View(LookupLastExecutionResultForServiceEvents(height2, eventType, &actualResults))
			require.NoError(t, err)
			require.Equal(t, result2.ID(), actualResults)

			err = db.View(LookupLastExecutionResultForServiceEvents(height3, eventType, &actualResults))
			require.NoError(t, err)
			require.Equal(t, result3.ID(), actualResults)
		})

		t.Run("different event type retrieve nothing", func(t *testing.T) {
			var actualResults flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEvents(height1, flow.ServiceEventSetup, &actualResults))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("finds highest but not higher than given", func(t *testing.T) {

			var actualResults flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEvents(height3-1, eventType, &actualResults))
			require.NoError(t, err)
			require.Equal(t, result2.ID(), actualResults)
		})

		t.Run("finds highest", func(t *testing.T) {

			var actualResults flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEvents(height3+1, eventType, &actualResults))
			require.NoError(t, err)
			require.Equal(t, result3.ID(), actualResults)
		})

		t.Run("height below lowest entry returns nothing", func(t *testing.T) {
			var actualResults flow.Identifier
			err := db.View(LookupLastExecutionResultForServiceEvents(height1-1, flow.ServiceEventSetup, &actualResults))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

	})
}
