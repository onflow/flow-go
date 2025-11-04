package execution_data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func TestBlockExecutionDataHelpers(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	chunkDatas := g.ChunkExecutionDatas().List(5)
	blockData := g.BlockExecutionDatas().Fixture(
		fixtures.BlockExecutionData.WithChunkExecutionDatas(chunkDatas...),
	)

	expectedStardardChunks := chunkDatas[:len(chunkDatas)-1]
	expectedStandardCollections := make([]*flow.Collection, len(expectedStardardChunks))
	for i, chunk := range expectedStardardChunks {
		expectedStandardCollections[i] = chunk.Collection
	}

	expectedSystemChunk := chunkDatas[len(chunkDatas)-1]
	expectedSystemCollection := expectedSystemChunk.Collection

	t.Run("StandardChunks", func(t *testing.T) {
		assert.Equal(t, blockData.StandardChunks(), expectedStardardChunks)
	})

	t.Run("StandardCollections", func(t *testing.T) {
		assert.Equal(t, blockData.StandardCollections(), expectedStandardCollections)
	})

	t.Run("SystemChunk", func(t *testing.T) {
		assert.Equal(t, blockData.SystemChunk(), expectedSystemChunk)
	})

	t.Run("SystemCollection", func(t *testing.T) {
		assert.Equal(t, blockData.SystemCollection(), expectedSystemCollection)
	})
}

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
