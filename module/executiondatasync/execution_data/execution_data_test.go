package execution_data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

func TestConvertTransactionResults(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		assert.Nil(t, execution_data.ConvertTransactionResults(nil))

		var results []flow.TransactionResult
		assert.Nil(t, execution_data.ConvertTransactionResults(results))

		results = make([]flow.TransactionResult, 0)
		assert.Nil(t, execution_data.ConvertTransactionResults(results))
	})

	t.Run("non-empty", func(t *testing.T) {
		results := []flow.TransactionResult{
			{
				TransactionID:   flow.Identifier{1, 2, 3},
				ComputationUsed: 100,
				MemoryUsed:      1000,
			},
			{
				TransactionID:   flow.Identifier{4, 5, 6},
				ComputationUsed: 200,
				MemoryUsed:      2000,
				ErrorMessage:    "some error",
			},
		}
		expected := []flow.LightTransactionResult{
			{
				TransactionID:   flow.Identifier{1, 2, 3},
				ComputationUsed: 100,
				Failed:          false,
			},
			{
				TransactionID:   flow.Identifier{4, 5, 6},
				ComputationUsed: 200,
				Failed:          true,
			},
		}

		converted := execution_data.ConvertTransactionResults(results)
		assert.Equal(t, len(results), len(converted))

		for i, e := range expected {
			assert.Equal(t, e, converted[i])
			assert.Equal(t, e.TransactionID, converted[i].TransactionID)
			assert.Equal(t, e.ComputationUsed, converted[i].ComputationUsed)
			assert.Equal(t, e.Failed, converted[i].Failed)
		}
	})
}
