package hasher_test

import (
	"testing"

	"github.com/onflow/flow-go/ledger/common/hasher"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Test_LedgerHasherGetDefaultHashForHeight tests getting default hash for given heights
func Test_LedgerHasherGetDefaultHashForHeight(t *testing.T) {
	lh := hasher.NewLedgerHasher(hasher.DefaultHashMethod)
	defaultLeafHash := lh.HashLeaf([]byte("default:"), []byte{})
	assert.Equal(t, defaultLeafHash, lh.GetDefaultHashForHeight(0))

	l1 := lh.HashInterNode(lh.GetDefaultHashForHeight(0), lh.GetDefaultHashForHeight(0))
	assert.Equal(t, l1, lh.GetDefaultHashForHeight(1))

	l2 := lh.HashInterNode(l1, l1)
	assert.Equal(t, l2, lh.GetDefaultHashForHeight(2))
}

func Test_ComputeCompactValue(t *testing.T) {

	lh := hasher.NewLedgerHasher(hasher.DefaultHashMethod)
	kp1 := ledger.NewKeyPart(uint16(1), []byte("key part 1"))
	k := ledger.NewKey([]ledger.KeyPart{kp1})

	v := ledger.Value([]byte{'A'})
	p := ledger.NewPayload(k, v)

	// 00000101
	path := utils.OneBytePath(5)
	nodeHeight := 3
	l0 := lh.HashLeaf(path, v)
	l1 := lh.HashInterNode(lh.GetDefaultHashForHeight(0), l0)
	l2 := lh.HashInterNode(l1, lh.GetDefaultHashForHeight(1))
	l3 := lh.HashInterNode(lh.GetDefaultHashForHeight(2), l2)
	assert.Equal(t, l3, lh.ComputeCompactValue(path, p, nodeHeight))
}
