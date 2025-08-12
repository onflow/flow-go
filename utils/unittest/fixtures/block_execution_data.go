package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// BlockExecutionDataGenerator generates block execution data with consistent randomness.
type BlockExecutionDataGenerator struct {
	identifierGen         *IdentifierGenerator
	chunkExecutionDataGen *ChunkExecutionDataGenerator
}

// blockExecutionDataConfig holds the configuration for block execution data generation.
type blockExecutionDataConfig struct {
	blockID             flow.Identifier
	chunkExecutionDatas []*execution_data.ChunkExecutionData
}

// WithBlockID returns an option to set the block ID for the block execution data.
func (g *BlockExecutionDataGenerator) WithBlockID(blockID flow.Identifier) func(*blockExecutionDataConfig) {
	return func(config *blockExecutionDataConfig) {
		config.blockID = blockID
	}
}

// WithChunkExecutionDatas returns an option to set the chunk execution datas for the block execution data.
func (g *BlockExecutionDataGenerator) WithChunkExecutionDatas(chunks ...*execution_data.ChunkExecutionData) func(*blockExecutionDataConfig) {
	return func(config *blockExecutionDataConfig) {
		config.chunkExecutionDatas = chunks
	}
}

// Fixture generates block execution data with optional configuration.
func (g *BlockExecutionDataGenerator) Fixture(t testing.TB, opts ...func(*blockExecutionDataConfig)) *execution_data.BlockExecutionData {
	config := &blockExecutionDataConfig{
		blockID:             g.identifierGen.Fixture(t),
		chunkExecutionDatas: []*execution_data.ChunkExecutionData{},
	}

	for _, opt := range opts {
		opt(config)
	}

	return &execution_data.BlockExecutionData{
		BlockID:             config.blockID,
		ChunkExecutionDatas: config.chunkExecutionDatas,
	}
}

// List generates a list of block execution data.
func (g *BlockExecutionDataGenerator) List(t testing.TB, n int, opts ...func(*blockExecutionDataConfig)) []*execution_data.BlockExecutionData {
	list := make([]*execution_data.BlockExecutionData, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}

// BlockExecutionDataEntityGenerator generates block execution data entities with consistent randomness.
type BlockExecutionDataEntityGenerator struct {
	*BlockExecutionDataGenerator
}

// Fixture generates a block execution data entity with optional configuration.
func (g *BlockExecutionDataEntityGenerator) Fixture(t testing.TB, opts ...func(*blockExecutionDataConfig)) *execution_data.BlockExecutionDataEntity {
	execData := g.BlockExecutionDataGenerator.Fixture(t, opts...)
	return execution_data.NewBlockExecutionDataEntity(g.identifierGen.Fixture(t), execData)
}

// List generates a list of block execution data entities.
func (g *BlockExecutionDataEntityGenerator) List(t testing.TB, n int, opts ...func(*blockExecutionDataConfig)) []*execution_data.BlockExecutionDataEntity {
	list := make([]*execution_data.BlockExecutionDataEntity, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
