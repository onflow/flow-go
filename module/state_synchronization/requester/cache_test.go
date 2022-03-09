package requester

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsert(t *testing.T) {
	var expected []uint64

	insertCount := uint64(100000)
	heights := make(map[uint64]struct{}, insertCount)
	for i := uint64(0); i < insertCount; i++ {
		heights[i] = struct{}{}
		expected = append([]uint64{i}, expected...)
	}

	// use the non-deterministic map iteration order to test out of order inserts
	actual := make([]uint64, 0, len(heights))
	for h := range heights {
		actual = insert(actual, h)
	}

	assert.Equal(t, expected, actual)
}

func TestRemoveAll(t *testing.T) {
	removeCount := uint64(100000)

	t.Run("remove random order", func(t *testing.T) {
		var actual []uint64

		heights := make(map[uint64]struct{}, removeCount)
		for i := uint64(0); i < removeCount; i++ {
			heights[i] = struct{}{}
			actual = append([]uint64{i}, actual...)
		}

		// use the non-deterministic map iteration order to test out of order removes
		for h := range heights {
			actual = remove(actual, h)
		}

		assert.Equal(t, []uint64{}, actual)
	})

	t.Run("remove descending order (front to back)", func(t *testing.T) {
		var actual []uint64

		for i := uint64(0); i < removeCount; i++ {
			actual = append([]uint64{i}, actual...)
		}

		for i := removeCount - 1; i >= 0; i-- {
			actual = remove(actual, i)

			// explicitly break, otherwise uint will overflow to a positive number
			if i == 0 {
				break
			}
		}

		assert.Equal(t, []uint64{}, actual)
	})

	t.Run("remove ascending order (back to front)", func(t *testing.T) {
		var actual []uint64

		for i := uint64(0); i < removeCount; i++ {
			actual = append([]uint64{i}, actual...)
		}

		for i := uint64(0); i < removeCount; i++ {
			actual = remove(actual, i)
		}

		assert.Equal(t, []uint64{}, actual)
	})
}
