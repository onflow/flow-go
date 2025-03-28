package unittest

import "github.com/onflow/flow-go/model/flow"

// MockEntity implements a bare minimum entity for sake of test.
type MockEntity struct {
	Identifier flow.Identifier
	Nonce      uint64
}

//
//func (m MockEntity) ID() flow.Identifier {
//	return m.Identifier
//}
//
//func (m MockEntity) Checksum() flow.Identifier {
//	return m.Identifier
//}

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
