package flow_test

import (
	"math/rand"
	"testing"

	clone "github.com/huandu/go-clone/generic" //nolint:goimports

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// TestQuorumCertificateID_Malleability confirms that the QuorumCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestQuorumCertificateID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.QuorumCertificateFixture())
}

// TestQuorumCertificate_Equals verifies the correctness of the Equals method on QuorumCertificates.
// It checks that QuorumCertificates are considered equal if and only if all fields match.
func TestQuorumCertificate_Equals(t *testing.T) {
	// Create two QuorumCertificate with random but different values.
	qc1 := unittest.QuorumCertificateFixture()
	qc2 := unittest.QuorumCertificateFixture()
	// Initially, all fields are different, so the objects should not be equal.
	require.False(t, qc1.Equals(qc2))

	// List of mutations to apply on timeout1 to gradually make it equal to qc2
	// (excluding TimeoutTick).
	mutations := []func(){
		func() {
			qc1.View = qc2.View
		}, func() {
			qc1.BlockID = qc2.BlockID
		}, func() {
			qc1.SignerIndices = clone.Clone(qc2.SignerIndices) // deep copy
		}, func() {
			qc1.SigData = clone.Clone(qc2.SigData) // deep copy
		},
	}

	// Shuffle the order of mutations
	rand.Shuffle(len(mutations), func(i, j int) {
		mutations[i], mutations[j] = mutations[j], mutations[i]
	})

	// Apply each mutation one at a time, except the last.
	// After each step, the objects should still not be equal.
	for _, mutation := range mutations[:len(mutations)-1] {
		mutation()
		require.False(t, qc1.Equals(qc2))
	}

	// Apply the final mutation; now all relevant fields should match, so the objects must be equal.
	mutations[len(mutations)-1]()
	require.True(t, qc1.Equals(qc2))
}

// TestQuorumCertificate_Equals_Nil verifies the behavior of the Equals method when either
// or both the receiver and the function input are nil
func TestQuorumCertificate_Equals_Nil(t *testing.T) {
	var nilQC *flow.QuorumCertificate
	qc := unittest.QuorumCertificateFixture()
	t.Run("nil receiver", func(t *testing.T) {
		require.False(t, nilQC.Equals(qc))
	})
	t.Run("nil input", func(t *testing.T) {
		require.False(t, qc.Equals(nilQC))
	})
	t.Run("both nil", func(t *testing.T) {
		require.True(t, nilQC.Equals(nil))
	})
}
