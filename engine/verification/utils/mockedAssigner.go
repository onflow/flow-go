package utils

import (
	"fmt"

	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
)

type MockAssignment struct {
	me         flow.Identifier
	isAssigned func(index uint64) bool
}

func NewMockAssignment(id flow.Identifier, f func(index uint64) bool) *MockAssignment {
	return &MockAssignment{me: id, isAssigned: f}
}

// Assign assigns all input chunks to the verifier node
func (m *MockAssignment) Assign(ids flow.IdentityList, result *flow.ExecutionResult) (*chmodel.Assignment, error) {
	// , rng random.Rand is never used.
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
