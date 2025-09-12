package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// BlockExecutionDataGenerator generates block execution data with consistent randomness.
type BlockExecutionDataGenerator struct {
	identifierGen         *IdentifierGenerator
	chunkExecutionDataGen *ChunkExecutionDataGenerator
}

func NewBlockExecutionDataGenerator(
	identifierGen *IdentifierGenerator,
	chunkExecutionDataGen *ChunkExecutionDataGenerator,
) *BlockExecutionDataGenerator {
	return &BlockExecutionDataGenerator{
		identifierGen:         identifierGen,
		chunkExecutionDataGen: chunkExecutionDataGen,
	}
}

// WithBlockID is an option that sets the BlockID for the block execution data.
func (g *BlockExecutionDataGenerator) WithBlockID(blockID flow.Identifier) func(*execution_data.BlockExecutionData) {
	return func(blockExecutionData *execution_data.BlockExecutionData) {
		blockExecutionData.BlockID = blockID
	}
}

// WithChunkExecutionDatas is an option that sets the ChunkExecutionDatas for the block execution data.
func (g *BlockExecutionDataGenerator) WithChunkExecutionDatas(chunks ...*execution_data.ChunkExecutionData) func(*execution_data.BlockExecutionData) {
	return func(blockExecutionData *execution_data.BlockExecutionData) {
		blockExecutionData.ChunkExecutionDatas = chunks
	}
}

// Fixture generates a [execution_data.BlockExecutionData] with random data based on the provided options.
func (g *BlockExecutionDataGenerator) Fixture(opts ...func(*execution_data.BlockExecutionData)) *execution_data.BlockExecutionData {
	blockExecutionData := &execution_data.BlockExecutionData{
		BlockID:             g.identifierGen.Fixture(),
		ChunkExecutionDatas: []*execution_data.ChunkExecutionData{},
	}

	for _, opt := range opts {
		opt(blockExecutionData)
	}

	return blockExecutionData
}

// List generates a list of [execution_data.BlockExecutionData].
func (g *BlockExecutionDataGenerator) List(n int, opts ...func(*execution_data.BlockExecutionData)) []*execution_data.BlockExecutionData {
	list := make([]*execution_data.BlockExecutionData, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// BlockExecutionDataEntityGenerator generates [execution_data.BlockExecutionDataEntity] with consistent randomness.
type BlockExecutionDataEntityGenerator struct {
	*BlockExecutionDataGenerator
}

func NewBlockExecutionDataEntityGenerator(
	blockExecutionDataGen *BlockExecutionDataGenerator,
) *BlockExecutionDataEntityGenerator {
	return &BlockExecutionDataEntityGenerator{
		BlockExecutionDataGenerator: blockExecutionDataGen,
	}
}

// Fixture generates a [execution_data.BlockExecutionDataEntity] with random data based on the provided options.
func (g *BlockExecutionDataEntityGenerator) Fixture(opts ...func(*execution_data.BlockExecutionData)) *execution_data.BlockExecutionDataEntity {
	execData := g.BlockExecutionDataGenerator.Fixture(opts...)
	return execution_data.NewBlockExecutionDataEntity(g.identifierGen.Fixture(), execData)
}

// List generates a list of [execution_data.BlockExecutionDataEntity].
func (g *BlockExecutionDataEntityGenerator) List(n int, opts ...func(*execution_data.BlockExecutionData)) []*execution_data.BlockExecutionDataEntity {
	list := make([]*execution_data.BlockExecutionDataEntity, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
