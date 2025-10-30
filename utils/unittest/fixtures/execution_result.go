package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionResult is the default options factory for [flow.ExecutionResult] generation.
var ExecutionResult executionResultFactory

type executionResultFactory struct{}

type ExecutionResultOption func(*ExecutionResultGenerator, *flow.ExecutionResult)

// WithPreviousResultID is an option that sets the `PreviousResultID` of the execution result.
func (f executionResultFactory) WithPreviousResultID(previousResultID flow.Identifier) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.PreviousResultID = previousResultID
	}
}

// WithBlockID is an option that sets the `BlockID` of the execution result.
func (f executionResultFactory) WithBlockID(blockID flow.Identifier) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.BlockID = blockID
		for _, chunk := range result.Chunks {
			chunk.BlockID = blockID
		}
	}
}

// WithChunks is an option that sets the `Chunks` of the execution result.
func (f executionResultFactory) WithChunks(chunks flow.ChunkList) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.Chunks = chunks
	}
}

// WithServiceEvents is an option that sets the `ServiceEvents` of the execution result.
func (f executionResultFactory) WithServiceEvents(serviceEvents flow.ServiceEventList) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.ServiceEvents = serviceEvents
	}
}

// WithExecutionDataID is an option that sets the `ExecutionDataID` of the execution result.
func (f executionResultFactory) WithExecutionDataID(executionDataID flow.Identifier) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.ExecutionDataID = executionDataID
	}
}

// WithBlock is an option that sets the `BlockID` of the execution result and all configured `Chunks`
func (f executionResultFactory) WithBlock(block *flow.Block) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.BlockID = block.ID()
		for _, chunk := range result.Chunks {
			chunk.BlockID = block.ID()
		}
	}
}

// WithPreviousResult is an option that sets the `PreviousResultID` of the execution result, and
// adjusts the `StartState` of the first chunk to match the final state of the previous result.
func (f executionResultFactory) WithPreviousResult(previousResult *flow.ExecutionResult) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.PreviousResultID = previousResult.ID()
		finalState, err := previousResult.FinalStateCommitment()
		NoError(err)
		result.Chunks[0].StartState = finalState
	}
}

// WithFinalState is an option that sets the `EndState` of the last chunk.
func (f executionResultFactory) WithFinalState(finalState flow.StateCommitment) ExecutionResultOption {
	return func(g *ExecutionResultGenerator, result *flow.ExecutionResult) {
		result.Chunks[len(result.Chunks)-1].EndState = finalState
	}
}

// ExecutionResultGenerator generates execution results with consistent randomness.
type ExecutionResultGenerator struct {
	executionResultFactory

	random           *RandomGenerator
	identifiers      *IdentifierGenerator
	chunks           *ChunkGenerator
	serviceEvents    *ServiceEventGenerator
	stateCommitments *StateCommitmentGenerator
}

// NewExecutionResultGenerator creates a new ExecutionResultGenerator.
func NewExecutionResultGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	chunks *ChunkGenerator,
	serviceEvents *ServiceEventGenerator,
	stateCommitments *StateCommitmentGenerator,
) *ExecutionResultGenerator {
	return &ExecutionResultGenerator{
		random:           random,
		identifiers:      identifiers,
		chunks:           chunks,
		serviceEvents:    serviceEvents,
		stateCommitments: stateCommitments,
	}
}

// Fixture generates a [flow.ExecutionResult] with random data based on the provided options.
func (g *ExecutionResultGenerator) Fixture(opts ...ExecutionResultOption) *flow.ExecutionResult {
	blockID := g.identifiers.Fixture()
	chunks := g.chunks.List(g.random.IntInRange(1, 4), Chunk.WithBlockID(blockID))
	serviceEventCount := 0
	for _, chunk := range chunks {
		serviceEventCount += int(chunk.ServiceEventCount)
	}
	result := &flow.ExecutionResult{
		PreviousResultID: g.identifiers.Fixture(),
		BlockID:          blockID,
		Chunks:           chunks,
		ServiceEvents:    g.serviceEvents.List(serviceEventCount),
		ExecutionDataID:  g.identifiers.Fixture(),
	}

	for _, opt := range opts {
		opt(g, result)
	}

	Assertf(len(result.Chunks) > 0, "there must be at least one chunk")

	return result
}

// List generates a list of [flow.ExecutionResult].
func (g *ExecutionResultGenerator) List(n int, opts ...ExecutionResultOption) []*flow.ExecutionResult {
	list := make([]*flow.ExecutionResult, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
