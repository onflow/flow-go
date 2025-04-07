package stdmap_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTimesPool(t *testing.T) {
	id := unittest.IdentifierFixture()
	ti := time.Now()

	pool := stdmap.NewTimes(3)

	t.Run("should be able to add", func(t *testing.T) {
		added := pool.Add(id, ti)
		assert.True(t, added)
	})

	t.Run("should be able to get", func(t *testing.T) {
		got, exists := pool.Get(id)
		assert.True(t, exists)
		assert.Equal(t, ti, got)
	})

	t.Run("should be able to remove", func(t *testing.T) {
		ok := pool.Remove(id)
		assert.True(t, ok)
	})
}
