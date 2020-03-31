// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestResultsInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(t *testing.T, db *badger.DB) {
		expected := unittest.ExecutionResultFixture()

		err := db.Update(InsertExecutionResult(expected))
		require.Nil(t, err)

		var actual flow.ExecutionResult
		err = db.View(RetrieveExecutionResult(expected.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}
