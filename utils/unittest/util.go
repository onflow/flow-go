package unittest

import (
	"github.com/onflow/flow-go/model/flow"
)

// MockEntity is a simple struct used for testing mempools that work with entities.
// It provides an Identifier and a Nonce field to simulate basic entity behavior.
// This allows for controlled testing of how mempools handle entity storage and updates.
type MockEntity struct {
	Identifier flow.Identifier
	Nonce      uint64
}

func EntityListFixture(n uint) []*MockEntity {
	list := make([]*MockEntity, 0, n)
	for range n {
		list = append(list, MockEntityFixture())
	}
	return list
}

func MockEntityFixture() *MockEntity {
	return &MockEntity{Identifier: IdentifierFixture()}
}
