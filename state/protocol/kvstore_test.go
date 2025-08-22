package protocol

import (
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	clone "github.com/huandu/go-clone/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecutionMeteringParameters_EncodeRLP tests properties of the custom RLP encoding logic.
func TestExecutionMeteringParameters_EncodeRLP(t *testing.T) {
	params1 := &ExecutionMeteringParameters{
		ExecutionMemoryLimit:   rand.Uint64(),
		ExecutionMemoryWeights: make(map[uint32]uint64, 10),
		ExecutionEffortWeights: make(map[uint32]uint64, 10),
	}
	for range 10 {
		params1.ExecutionMemoryWeights[rand.Uint32()] = rand.Uint64()
		params1.ExecutionEffortWeights[rand.Uint32()] = rand.Uint64()
	}

	t.Run("deterministic encoding", func(t *testing.T) {
		enc1, err := rlp.EncodeToBytes(params1)
		require.NoError(t, err)
		enc2, err := rlp.EncodeToBytes(params1)
		require.NoError(t, err)
		assert.Equal(t, enc1, enc2)
	})
	t.Run("unique encoding", func(t *testing.T) {
		params2 := clone.Clone(params1)
		for k, v := range params2.ExecutionMemoryWeights {
			params2.ExecutionMemoryWeights[k] = v + 1
			assert.NotEqual(t, params1.ExecutionMemoryWeights[k], params2.ExecutionMemoryWeights[k])
			break
		}
		enc1, err := rlp.EncodeToBytes(params1)
		require.NoError(t, err)
		enc2, err := rlp.EncodeToBytes(params2)
		require.NoError(t, err)
		assert.NotEqual(t, enc1, enc2)
	})
}
