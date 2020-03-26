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
		_ = pool.Add(item1)
		_ = pool.Add(item2)

		t.Run("should be able to get size", func(t *testing.T) {
			size := pool.Size()
			assert.EqualValues(t, uint(2), size)
		})

		t.Run("should be able to get first", func(t *testing.T) {
			gotItem, err := pool.ByID(item1.ID())
			assert.NoError(t, err)
			assert.Equal(t, item1, gotItem)
		})

		t.Run("should be able to remove first", func(t *testing.T) {
			ok := pool.Rem(item1.ID())
			assert.True(t, ok)
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
