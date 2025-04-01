package unittest

import (
	"github.com/onflow/flow-go/model/flow"
)

// MockEntity is a simple struct used for testing mempools constrained to Identifier-typed keys.
// The Identifier field is the key used to store the entity in the mempool. 
// The Nonce field can be used to simulate modifying the entity value stored in the mempool,
// for example via `Adjust`/`AdjustWithInit` methods.
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
