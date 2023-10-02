package unittest

import (
	"crypto/rand"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"testing"
)

// MockEntity implements a bare minimum entity for sake of test.
type MockEntity struct {
	b  []byte
	id flow.Identifier
}

func NewMockEntity(id flow.Identifier) *MockEntity {
	return &MockEntity{
		b:  id[:],
		id: id,
	}
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
		id := IdentifierFixture()
		list = append(list, &MockEntity{
			id: id,
			b:  id[:],
		})
	}

	return list
}

func MockEntityFixture() *MockEntity {
	id := IdentifierFixture()
	b := id[:]
	return &MockEntity{
		b:  b,
		id: id,
	}
}

func MockEntityFixtureWithSize(t testing.TB, size int) *MockEntity {
	b := make([]byte, size)
	n, err := rand.Read(b)
	require.NoError(t, err)
	require.Equal(t, size, n)
	return &MockEntity{
		b:  b,
		id: flow.MakeID(b),
	}
}

func MockEntityFixtureListWithSize(t testing.TB, count int, size int) []*MockEntity {
	entities := make([]*MockEntity, 0, count)
	for i := 0; i < count; i++ {
		e := MockEntityFixtureWithSize(t, size)
		require.NotContainsf(t, entities, e, "entity %d already exists", i)
		entities = append(entities, e)
	}
	return entities
}

func MockEntityListFixture(count int) []*MockEntity {
	entities := make([]*MockEntity, 0, count)
	for i := 0; i < count; i++ {
		entities = append(entities, MockEntityFixture())
	}
	return entities
}
