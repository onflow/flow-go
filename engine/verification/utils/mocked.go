package utils

import (
	"fmt"

	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
)

// MockAssigner ...
type MockAssigner struct {
	me         flow.Identifier
	isAssigned func(index uint64) bool
}

// NewMockAssigner ...
func NewMockAssigner(id flow.Identifier, f func(index uint64) bool) *MockAssigner {
	return &MockAssigner{me: id, isAssigned: f}
}

// Assign assigns all input chunks to the verifier node
func (m *MockAssigner) Assign(ids flow.IdentityList, result *flow.ExecutionResult) (*chmodel.Assignment, error) {
	if len(result.Chunks) == 0 {
		return nil, fmt.Errorf("assigner called with empty chunk list")
	}
	a := chmodel.NewAssignment()
	for _, c := range result.Chunks {
		if m.isAssigned(c.Index) {
			a.Add(c, flow.IdentifierList{m.me})
		}
	}

	return a, nil
}

// GetAssignedChunks ...
func (m *MockAssigner) GetAssignedChunks(verifierID flow.Identifier, assignment *chmodel.Assignment, result *flow.ExecutionResult) (flow.ChunkList, error) {
	// indices of chunks assigned to verifier
	chunkIndices := assignment.ByNodeID(verifierID)

	// chunks keeps the list of chunks assigned to the verifier
	chunks := make(flow.ChunkList, 0, len(chunkIndices))
	for _, index := range chunkIndices {
		chunk, ok := result.Chunks.ByIndex(index)
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", index)
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
