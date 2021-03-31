package chunkrequester

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// when maxAttempt is set to 3, CanTry will only return true for the first 3 times.
func TestCanTry(t *testing.T) {
	t.Run("maxAttempt=3", func(t *testing.T) {
		maxAttempt := 3
		chunks := NewChunks(10)
		c := unittest.ChunkFixture(flow.Identifier{0x11}, 0)
		c.Index = 0
		chunk := NewChunkStatus(c, flow.Identifier{0xaa}, 3, []flow.Identifier{}, []flow.Identifier{})
		chunks.Add(chunk)
		results := []bool{}
		for i := 0; i < 5; i++ {
			results = append(results, CanTry(maxAttempt, chunk))
			chunks.IncrementAttempt(chunk.ID())
		}
		require.Equal(t, []bool{true, true, true, false, false}, results)
	})
}
