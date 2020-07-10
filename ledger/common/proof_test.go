package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger/common"
)

// Test_ProofVerify tests proof verification
func Test_ProofVerify(t *testing.T) {
	p, sc := common.ProofFixture()
	require.True(t, common.VerifyProof(p, sc, 2))
}

// Test_BatchProofVerify tests batch proof verification
func Test_BatchProofVerify(t *testing.T) {
	bp, sc := common.BatchProofFixture()
	require.True(t, common.VerifyBatchProof(bp, sc, 2))
}
