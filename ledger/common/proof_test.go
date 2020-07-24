package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/common/utils"
)

// Test_ProofVerify tests proof verification
func Test_TrieProofVerify(t *testing.T) {
	p, sc := utils.TrieProofFixture()
	require.True(t, common.VerifyTrieProof(p, sc))
}

// Test_BatchProofVerify tests batch proof verification
func Test_TrieBatchProofVerify(t *testing.T) {
	bp, sc := utils.TrieBatchProofFixture()
	require.True(t, common.VerifyTrieBatchProof(bp, sc))
}
