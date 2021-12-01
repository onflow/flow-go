package backdata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestArrayBackData_Add(t *testing.T) {
	size := 10

	bd := NewArrayBackData(uint32(size), 1, RandomEjection)

	entities := unittest.EntityListFixture(uint(size))

	for i, e := range entities {
		// adding each element must be successful.
		require.True(t, bd.Add(e.ID(), e))

		// size of back data should be incremented by each addition.
		require.Equal(t, bd.Size(), uint(i+1))

		// entity should be placed at index i in back data
		require.Equal(t, bd.entities[i].entity, e)

		// sanity checks
		require.Equal(t, bd.buckets[0][0].keyIndex, uint64(1))
	}

}
