package stdmap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIncrementStatus evaluates that calling increment attempt several times updates the status attempts.
func TestIncrementStatus(t *testing.T) {
	increments := 5
	t.Run("5 times increment", func(t *testing.T) {
		requests := NewChunkRequests(10)

		status := &verification.ChunkRequestStatus{
			ChunkDataPackRequest: &verification.ChunkDataPackRequest{
				ChunkID:   unittest.IdentifierFixture(),
				Height:    0,
				Agrees:    []flow.Identifier{},
				Disagrees: []flow.Identifier{},
			},
			Attempt: 0,
		}

		// stores
		ok := requests.Add(status)
		require.True(t, ok)

		// increments attempts
		for i := 0; i < increments; i++ {
			ok = requests.IncrementAttempt(status.ID())
			require.True(t, ok)

		}

		// retrieves updated status
		status, ok = requests.ByID(status.ID())
		require.True(t, ok)
		require.Equal(t, status.Attempt, increments)
	})
}
