package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResults_InsertRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := unittest.ExecutionResultFixture()

		err := InsertExecutionResult(expected)(db)
		require.Nil(t, err)

		var actual flow.ExecutionResult
		err = RetrieveExecutionResult(expected.ID(), &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}
