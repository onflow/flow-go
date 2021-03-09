package proof_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/proof"
)

// Test_ProofVerify tests proof verification
func Test_TrieProofVerify(t *testing.T) {
	p, sc := ledger.TrieProofFixture()
	require.True(t, proof.VerifyTrieProof(p, sc))
}

// Test_BatchProofVerify tests batch proof verification
func Test_TrieBatchProofVerify(t *testing.T) {
	bp, sc := ledger.TrieBatchProofFixture()
	require.True(t, proof.VerifyTrieBatchProof(bp, sc))
}
