package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// BlockExecutionData is the default options factory for [execution_data.BlockExecutionData] generation.
var BlockExecutionData blockExecutionDataFactory

type blockExecutionDataFactory struct{}

type BlockExecutionDataOption func(*BlockExecutionDataGenerator, *execution_data.BlockExecutionData)

// WithBlockID is an option that sets the BlockID for the block execution data.
func (f blockExecutionDataFactory) WithBlockID(blockID flow.Identifier) BlockExecutionDataOption {
	return func(g *BlockExecutionDataGenerator, blockExecutionData *execution_data.BlockExecutionData) {
		blockExecutionData.BlockID = blockID
	}
}

// WithChunkExecutionDatas is an option that sets the ChunkExecutionDatas for the block execution data.
func (f blockExecutionDataFactory) WithChunkExecutionDatas(chunks ...*execution_data.ChunkExecutionData) BlockExecutionDataOption {
	return func(g *BlockExecutionDataGenerator, blockExecutionData *execution_data.BlockExecutionData) {
		blockExecutionData.ChunkExecutionDatas = chunks
	}
}

// BlockExecutionDataGenerator generates block execution data with consistent randomness.
type BlockExecutionDataGenerator struct {
	blockExecutionDataFactory

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

// Fixture generates a [execution_data.BlockExecutionData] with random data based on the provided options.
func (g *BlockExecutionDataGenerator) Fixture(opts ...BlockExecutionDataOption) *execution_data.BlockExecutionData {
	blockExecutionData := &execution_data.BlockExecutionData{
		BlockID:             g.identifierGen.Fixture(),
		ChunkExecutionDatas: []*execution_data.ChunkExecutionData{},
	}

	for _, opt := range opts {
		opt(g, blockExecutionData)
	}

	return blockExecutionData
}

// List generates a list of [execution_data.BlockExecutionData].
func (g *BlockExecutionDataGenerator) List(n int, opts ...BlockExecutionDataOption) []*execution_data.BlockExecutionData {
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
func (g *BlockExecutionDataEntityGenerator) Fixture(opts ...BlockExecutionDataOption) *execution_data.BlockExecutionDataEntity {
	execData := g.BlockExecutionDataGenerator.Fixture(opts...)
	return execution_data.NewBlockExecutionDataEntity(g.identifierGen.Fixture(), execData)
}

// List generates a list of [execution_data.BlockExecutionDataEntity].
func (g *BlockExecutionDataEntityGenerator) List(n int, opts ...BlockExecutionDataOption) []*execution_data.BlockExecutionDataEntity {
	list := make([]*execution_data.BlockExecutionDataEntity, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
