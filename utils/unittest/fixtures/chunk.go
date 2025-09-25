package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// Chunk is the default options factory for [flow.Chunk] generation.
var Chunk chunkFactory

type chunkFactory struct{}

type ChunkOption func(*ChunkGenerator, *flow.Chunk)

// WithIndex is an option that sets the `Index` of the chunk.
func (f chunkFactory) WithIndex(index uint64) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.Index = index
	}
}

// WithBlockID is an option that sets the `BlockID` of the chunk.
func (f chunkFactory) WithBlockID(blockID flow.Identifier) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.BlockID = blockID
	}
}

// WithCollectionIndex is an option that sets the `CollectionIndex` of the chunk.
func (f chunkFactory) WithCollectionIndex(collectionIndex uint) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.CollectionIndex = collectionIndex
	}
}

// WithStartState is an option that sets the `StartState` of the chunk.
func (f chunkFactory) WithStartState(startState flow.StateCommitment) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.StartState = startState
	}
}

// WithEndState is an option that sets the `EndState` of the chunk.
func (f chunkFactory) WithEndState(endState flow.StateCommitment) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.EndState = endState
	}
}

// WithEventCollection is an option that sets the `EventCollection` of the chunk.
func (f chunkFactory) WithEventCollection(eventCollection flow.Identifier) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.EventCollection = eventCollection
	}
}

// WithNumberOfTransactions is an option that sets the `NumberOfTransactions` of the chunk.
func (f chunkFactory) WithNumberOfTransactions(numTxs uint64) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.NumberOfTransactions = numTxs
	}
}

// WithTotalComputationUsed is an option that sets the `TotalComputationUsed` of the chunk.
func (f chunkFactory) WithTotalComputationUsed(computation uint64) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.TotalComputationUsed = computation
	}
}

// WithServiceEventCount is an option that sets the `ServiceEventCount` of the chunk.
func (f chunkFactory) WithServiceEventCount(count uint16) ChunkOption {
	return func(g *ChunkGenerator, chunk *flow.Chunk) {
		chunk.ServiceEventCount = count
	}
}

// ChunkGenerator generates chunks with consistent randomness.
type ChunkGenerator struct {
	chunkFactory

	random           *RandomGenerator
	identifiers      *IdentifierGenerator
	stateCommitments *StateCommitmentGenerator
}

// NewChunkGenerator creates a new ChunkGenerator.
func NewChunkGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	stateCommitments *StateCommitmentGenerator,
) *ChunkGenerator {
	return &ChunkGenerator{
		random:           random,
		identifiers:      identifiers,
		stateCommitments: stateCommitments,
	}
}

// Fixture generates a [flow.Chunk] with random data based on the provided options.
func (g *ChunkGenerator) Fixture(opts ...ChunkOption) *flow.Chunk {
	chunk := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      g.random.Uintn(10), // TODO: should CollectionIndex == Index?
			StartState:           g.stateCommitments.Fixture(),
			EventCollection:      g.identifiers.Fixture(),
			ServiceEventCount:    g.random.Uint16n(10),
			BlockID:              g.identifiers.Fixture(),
			TotalComputationUsed: g.random.Uint64InRange(1, 9999),
			NumberOfTransactions: g.random.Uint64InRange(1, 100),
		},
		Index:    g.random.Uint64(),
		EndState: g.stateCommitments.Fixture(),
	}

	for _, opt := range opts {
		opt(g, chunk)
	}

	return chunk
}

// List generates a list of [flow.Chunk].
func (g *ChunkGenerator) List(n int, opts ...ChunkOption) []*flow.Chunk {
	chunks := make([]*flow.Chunk, n)
	startState := g.stateCommitments.Fixture()
	for i := range n {
		endState := g.stateCommitments.Fixture()
		chunks[i] = g.Fixture(append(opts,
			Chunk.WithIndex(uint64(i)),
			Chunk.WithCollectionIndex(uint(i)),
			Chunk.WithStartState(startState),
			Chunk.WithEndState(endState),
		)...)
		startState = endState
	}
	return chunks
}
