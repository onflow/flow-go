package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertExecutionStateQuery tests converting a protobuf ExecutionStateQuery to Criteria.
func TestConvertExecutionStateQuery(t *testing.T) {
	t.Parallel()

	t.Run("non-nil query", func(t *testing.T) {
		executorIDs := unittest.IdentifierListFixture(5)
		agreeingCount := uint64(3)

		query := &entities.ExecutionStateQuery{
			AgreeingExecutorsCount: agreeingCount,
			RequiredExecutorIds:    convert.IdentifiersToMessages(executorIDs),
		}

		criteria := convert.NewCriteria(query)

		assert.Equal(t, uint(agreeingCount), criteria.AgreeingExecutorsCount)
		require.Equal(t, len(executorIDs), len(criteria.RequiredExecutors))
		for i, id := range executorIDs {
			assert.Equal(t, id, criteria.RequiredExecutors[i])
		}
	})

	t.Run("nil query", func(t *testing.T) {
		criteria := convert.NewCriteria(nil)

		expected := optimistic_sync.Criteria{}
		assert.Equal(t, expected, criteria)
	})

	t.Run("empty required executors", func(t *testing.T) {
		query := &entities.ExecutionStateQuery{
			AgreeingExecutorsCount: 5,
			RequiredExecutorIds:    [][]byte{},
		}

		criteria := convert.NewCriteria(query)

		assert.Equal(t, uint(5), criteria.AgreeingExecutorsCount)
		assert.Empty(t, criteria.RequiredExecutors)
	})

	t.Run("zero agreeing executors count", func(t *testing.T) {
		executorIDs := unittest.IdentifierListFixture(3)

		query := &entities.ExecutionStateQuery{
			AgreeingExecutorsCount: 0,
			RequiredExecutorIds:    convert.IdentifiersToMessages(executorIDs),
		}

		criteria := convert.NewCriteria(query)

		assert.Equal(t, uint(0), criteria.AgreeingExecutorsCount)
		assert.Equal(t, len(executorIDs), len(criteria.RequiredExecutors))
	})
}
