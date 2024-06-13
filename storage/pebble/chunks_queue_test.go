package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunksQueue(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		q := NewChunkQueue(db)
		inited, err := q.Init(0)
		require.NoError(t, err)
		require.True(t, inited)

		locator := &chunks.Locator{
			ChunkID: flow.Identifier{0x01},
			JobID:   flow.Identifier{0x02},
		}
	})
}
