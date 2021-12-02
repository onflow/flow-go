package backdata

import (
	"fmt"
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
		require.Equal(t, e, bd.entities[i].entity, fmt.Sprintf("mismatching stored entity: %d", i))

		// sanity checks
		// since we are below limit, elements should be added sequentially at bucket 0.
		// first added element has a key index of 1, since 0 means unused key index in implementation.
		require.Equal(t, bd.buckets[0][i].keyIndex, uint64(i+1))
	}

}
