package match

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

// when maxAttempt is set to 3, CanTry will only return true for the first 3 times.
func TestCanTry(t *testing.T) {
	t.Run("maxAttempt=3", func(t *testing.T) {
		maxAttempt := 3
		chunks := NewChunks(10)
		c := ChunkWithIndex(flow.Identifier{0x11}, 0)
		chunk := NewChunkStatus(c, flow.Identifier{0xaa}, flow.Identifier{0xbb})
		chunks.Add(chunk)
		results := []bool{}
		for i := 0; i < 5; i++ {
			results = append(results, CanTry(maxAttempt, chunk))
			chunks.IncrementAttempt(chunk.ID())
		}
		require.Equal(t, []bool{true, true, true, false, false}, results)
	})
}
