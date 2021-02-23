package hasher_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Test_GetDefaultHashForHeight tests getting default hash for given heights
func Test_GetDefaultHashForHeight(t *testing.T) {
	defaultLeafHash := common.HashLeaf([]byte("default:"), []byte{})
	assert.Equal(t, defaultLeafHash, common.GetDefaultHashForHeight(0))

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

	// 00000101
	path := utils.OneBytePath(5)
	nodeHeight := 3
	l0 := common.HashLeaf(path, v)
	l1 := common.HashInterNode(common.GetDefaultHashForHeight(0), l0)
	l2 := common.HashInterNode(l1, common.GetDefaultHashForHeight(1))
	l3 := common.HashInterNode(common.GetDefaultHashForHeight(2), l2)
	assert.Equal(t, l3, common.ComputeCompactValue(path, p, nodeHeight))
}
