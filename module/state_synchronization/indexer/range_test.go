package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Range_Init(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		index, err := NewSequentialIndexRange(10, 20)
		assert.NoError(t, err)
		assert.Equal(t, uint64(10), index.First())
		assert.Equal(t, uint64(20), index.Last())
	})

	t.Run("Invalid", func(t *testing.T) {
		index, err := NewSequentialIndexRange(20, 10)
		assert.EqualError(t, err, "first value can not be higher than the last value")
		assert.Nil(t, index)
	})
}

func Test_Increase(t *testing.T) {
	t.Run("Sequence", func(t *testing.T) {
		index, err := NewSequentialIndexRange(10, 20)
		assert.NoError(t, err)
		err = index.Increase(21)
		assert.NoError(t, err)
		err = index.Increase(22)
		assert.NoError(t, err)
		err = index.Increase(22)
		assert.NoError(t, err)
		err = index.Increase(23)
		assert.NoError(t, err)
	})

	t.Run("Invalid value", func(t *testing.T) {
		index, err := NewSequentialIndexRange(10, 20)
		assert.NoError(t, err)
		err = index.Increase(19)
		assert.EqualError(t, err, "value 19 should be equal or incremented by one from the last indexed value: 20: invalid index value")
		assert.ErrorIs(t, err, ErrIndexValue)
		err = index.Increase(23)
		assert.EqualError(t, err, "value 23 should be equal or incremented by one from the last indexed value: 20: invalid index value")
		assert.ErrorIs(t, err, ErrIndexValue)
	})

	t.Run("Can Increase", func(t *testing.T) {
		index, err := NewSequentialIndexRange(10, 20)
		assert.NoError(t, err)
		ok, err := index.CanIncrease(21)
		assert.True(t, ok)
		assert.Nil(t, err)
	})
}

func Test_Contained(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		index, err := NewSequentialIndexRange(10, 20)
		assert.NoError(t, err)
		nums := []uint64{15, 10, 20}
		for _, v := range nums {
			ok, err := index.Contained(v)
			assert.True(t, ok)
			assert.Nil(t, err)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		index, err := NewSequentialIndexRange(10, 20)
		assert.NoError(t, err)
		ok, err := index.Contained(22)
		assert.EqualError(t, err, "value 22 is out of boundary [10 - 20]: the value is not within the indexed interval")
		assert.False(t, ok)
	})
}
