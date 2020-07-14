package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger/common"
)

// Test_ProofVerify tests proof verification
func Test_TrieProofVerify(t *testing.T) {
	p, sc := common.TrieProofFixture()
	require.True(t, common.VerifyTrieProof(p, sc, 2))
}

// Test_BatchProofVerify tests batch proof verification
func Test_TrieBatchProofVerify(t *testing.T) {
	bp, sc := common.TrieBatchProofFixture()
	require.True(t, common.VerifyTrieBatchProof(bp, sc, 2))
}
