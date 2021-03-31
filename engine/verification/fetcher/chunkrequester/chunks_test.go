package chunkrequester

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCanTry evaluates that when maxAttempt is set to 3, canTry will only return true for the first 3 times.
func TestCanTry(t *testing.T) {
	t.Run("maxAttempt=3", func(t *testing.T) {
		maxAttempt := 3
		requests := NewChunkRequests(10)
		c := unittest.ChunkFixture(flow.Identifier{0x11}, 0)
		c.Index = 0
		status := ChunkRequestStatus{
			Request: &fetcher.ChunkDataPackRequest{
				ChunkID:   unittest.IdentifierFixture(),
				Height:    0,
				Agrees:    []flow.Identifier{},
				Disagrees: []flow.Identifier{},
			},
			Attempt: 0,
		}

		requests.Add(&status)

		var results []bool
		for i := 0; i < 5; i++ {
			results = append(results, canTry(maxAttempt, status))
			requests.IncrementAttempt(status.ID())
		}
		require.Equal(t, []bool{true, true, true, false, false}, results)
	})
}
