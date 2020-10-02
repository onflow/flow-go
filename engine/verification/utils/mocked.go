package utils

import (
	"fmt"

	chmodel "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
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
func (m *MockAssigner) Assign(result *flow.ExecutionResult, blockID flow.Identifier) (*chmodel.Assignment, error) {
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
