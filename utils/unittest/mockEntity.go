package unittest

import (
	"github.com/onflow/flow-go/model/flow"
)

// MockEntity implements a bare minimum entity for sake of test.
type MockEntity struct {
	id flow.Identifier
}

func (m MockEntity) ID() flow.Identifier {
	return m.id
}

func (m MockEntity) Checksum() flow.Identifier {
	return m.id
}

func EntityListFixture(n uint) []*MockEntity {
	list := make([]*MockEntity, 0, n)

	for i := uint(0); i < n; i++ {
		list = append(list, &MockEntity{
			id: IdentifierFixture(),
		})
	}

	return list
}

func MockEntityFixture() *MockEntity {
	return &MockEntity{id: IdentifierFixture()}
}
