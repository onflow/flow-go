package common_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Test_GetDefaultHashForHeight tests getting default hash for given heights
func Test_GetDefaultHashForHeight(t *testing.T) {
	hasher := hash.NewSHA3_256()
	defaultLeafHash := hasher.ComputeHash([]byte("default:"))
	assert.Equal(t, []byte(defaultLeafHash), common.GetDefaultHashForHeight(0))

	l1 := common.HashInterNode(common.GetDefaultHashForHeight(0), common.GetDefaultHashForHeight(0))
	assert.Equal(t, l1, common.GetDefaultHashForHeight(1))

	l2 := common.HashInterNode(l1, l1)
	assert.Equal(t, l2, common.GetDefaultHashForHeight(2))
}

func Test_ComputeCompactValue(t *testing.T) {

	kp1 := ledger.NewKeyPart(uint16(1), []byte("key part 1"))
	k := ledger.NewKey([]ledger.KeyPart{kp1})

	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(k, v)

	// 00000101...00000000
	path := utils.PathByUint8(5)
	nodeHeight := 251
	h := common.HashLeaf(path, v)
	l := 0
	// exclude last 3 level
	for ; l < nodeHeight-3; l++ {
		h = common.HashInterNode(h, common.GetDefaultHashForHeight(l))
	}
	l1 := common.HashInterNode(common.GetDefaultHashForHeight(l), h)
	l2 := common.HashInterNode(l1, common.GetDefaultHashForHeight(l+1))
	l3 := common.HashInterNode(common.GetDefaultHashForHeight(l+2), l2)
	assert.Equal(t, l3, common.ComputeCompactValue(path, p, nodeHeight))
}

func TestHash(t *testing.T) {
	r := time.Now().UnixNano()
	rand.Seed(r)
	t.Logf("math rand seed is %d", r)

	t.Run("HashLeaf", func(t *testing.T) {
		path := make([]byte, 32)

		for i := 0; i < 5000; i++ {
			value := make([]byte, i)
			rand.Read(path)
			rand.Read(value)
			h := common.HashLeaf(path, value)

			hasher := hash.NewSHA3_256()
			_, _ = hasher.Write(path)
			_, _ = hasher.Write(value)
			expected := hasher.SumHash()
			assert.Equal(t, []byte(expected), []byte(h))
		}
	})

	t.Run("HashInterNode", func(t *testing.T) {
		h1 := make([]byte, 32)
		h2 := make([]byte, 32)

		for i := 0; i < 5000; i++ {
			rand.Read(h1)
			rand.Read(h2)
			h := common.HashInterNode(h1, h2)

			hasher := hash.NewSHA3_256()
			_, _ = hasher.Write(h1)
			_, _ = hasher.Write(h2)
			expected := hasher.SumHash()
			assert.Equal(t, []byte(expected), []byte(h))
		}
	})

	// Sanity check
	// TODO : remove once HashLeaf and HashInterNode are checked
	// to only receive 32-bytes inputs
	t.Run("HashLeaf", func(t *testing.T) {
		inp1 := make([]byte, 10)  // not 32
		inp2 := make([]byte, 100) // not 32
		rand.Read(inp1)
		rand.Read(inp2)
		hasher := hash.NewSHA3_256()
		_, _ = hasher.Write(inp1)
		_, _ = hasher.Write(inp2)
		expected := hasher.SumHash()

		h := common.HashLeaf(inp1, inp2)
		assert.Equal(t, []byte(expected), []byte(h))
		h = common.HashInterNode(inp1, inp2)
		assert.Equal(t, []byte(expected), []byte(h))
	})
}

func BenchmarkHash(b *testing.B) {

	h1 := make([]byte, 32)
	h2 := make([]byte, 32)
	rand.Read(h1)
	rand.Read(h2)

	// customized sha3 for ledger
	b.Run("LedgerSha3", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = common.HashInterNode(h1, h2)
		}
		b.StopTimer()
	})

	// flow crypto generic sha3
	b.Run("GenericSha3", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hasher := hash.NewSHA3_256()
			_, _ = hasher.Write(h1)
			_, _ = hasher.Write(h2)
			_ = hasher.SumHash()
		}
		b.StopTimer()
	})
}
