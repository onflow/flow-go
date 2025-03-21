package protocol

import (
	"math/rand"
	"testing"

	clone "github.com/huandu/go-clone/generic"
	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionMeteringParameters_EncodeRLP(t *testing.T) {
	params1 := &ExecutionMeteringParameters{
		ExecutionMemoryLimit: rand.Uint64(),
		ExecutionMemoryWeights: map[uint32]uint64{
			rand.Uint32(): rand.Uint64(),
			rand.Uint32(): rand.Uint64(),
			rand.Uint32(): rand.Uint64(),
		},
		ExecutionEffortWeights: map[uint32]uint64{
			rand.Uint32(): rand.Uint64(),
			rand.Uint32(): rand.Uint64(),
			rand.Uint32(): rand.Uint64(),
		},
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
