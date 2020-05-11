// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

type fake []byte

func (f fake) ID() flow.Identifier {
	return flow.HashToID(f)
}

func (f fake) Checksum() flow.Identifier {
	return flow.Identifier(sha256.Sum256(f))
}

func TestAddRem(t *testing.T) {
	item1 := fake("DEAD")
	item2 := fake("AGAIN")

	t.Run("should be able to add and rem", func(t *testing.T) {
		pool := NewBackend()
		added := pool.Add(item1)
		require.True(t, added)
		added = pool.Add(item2)
		require.True(t, added)

		t.Run("should be able to get size", func(t *testing.T) {
			size := pool.Size()
			assert.EqualValues(t, uint(2), size)
		})

		t.Run("should be able to get first", func(t *testing.T) {
			gotItem, exists := pool.ByID(item1.ID())
			assert.True(t, exists)
			assert.Equal(t, item1, gotItem)
		})

		t.Run("should be able to remove first", func(t *testing.T) {
			removed := pool.Rem(item1.ID())
			assert.True(t, removed)
			size := pool.Size()
			assert.EqualValues(t, uint(1), size)
		})

		t.Run("should be able to retrieve all", func(t *testing.T) {
			items := pool.All()
			require.Len(t, items, 1)
			assert.Equal(t, item2, items[0])
		})
	})
}

func TestAdjust(t *testing.T) {
	item1 := fake("DEAD")
	item2 := fake("AGAIN")

	t.Run("should not adjust if not exist", func(t *testing.T) {
		pool := NewBackend()
		_ = pool.Add(item1)

		// item2 doesn't exist
		updated := pool.Adjust(item2.ID(), func(old flow.Entity) flow.Entity {
			return item2
		})

		assert.False(t, updated)

		_, found := pool.ByID(item2.ID())
		assert.False(t, found)
	})

	t.Run("should not adjust if item didn't change", func(t *testing.T) {
		pool := NewBackend()
		_ = pool.Add(item1)

		updated := pool.Adjust(item1.ID(), func(old flow.Entity) flow.Entity {
			// item 1 exist, but got replaced with item1 again, nothing was changed
			return item1
		})

		assert.False(t, updated)

		value1, found := pool.ByID(item1.ID())
		assert.True(t, found)
		assert.Equal(t, value1, item1)
	})

	t.Run("should adjust if exists", func(t *testing.T) {
		pool := NewBackend()
		_ = pool.Add(item1)

		updated := pool.Adjust(item1.ID(), func(old flow.Entity) flow.Entity {
			// item 1 exist, got replaced with item2, the value was updated
			return item2
		})

		assert.True(t, updated)

		value2, found := pool.ByID(item2.ID())
		assert.True(t, found)
		assert.Equal(t, value2, item2)
	})
}
