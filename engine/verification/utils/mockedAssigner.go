package utils

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/random"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
)

type MockAssigner struct {
	me         flow.Identifier
	isAssigned func(index uint64) bool
}

func NewMockAssigner(id flow.Identifier, f func(index uint64) bool) *MockAssigner {
	return &MockAssigner{me: id, isAssigned: f}
}

// Assign assigns all input chunks to the verifier node
func (m *MockAssigner) Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chmodel.Assignment, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("assigner called with empty chunk list")
	}
	a := chmodel.NewAssignment()
	for _, c := range chunks {
		if m.isAssigned(c.Index) {
			a.Add(c, flow.IdentifierList{m.me})
		}
	}

	return a, nil
}

// IsAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func IsAssigned(index uint64) bool {
	return index%2 == 0
}
