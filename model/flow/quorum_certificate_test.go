package flow_test

import (
	"math/rand"
	"testing"

	clone "github.com/huandu/go-clone/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestQuorumCertificateID_Malleability confirms that the QuorumCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestQuorumCertificateID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.QuorumCertificateFixture())
}

// TestQuorumCertificate_Equals verifies the correctness of the Equals method on QuorumCertificates.
// It checks that QuorumCertificates are considered equal if and only if all fields match.
func TestQuorumCertificate_Equals(t *testing.T) {
	// Create two QuorumCertificates with random but different values. Note: random selection for `SignerIndices` has limited variability and
	// yields sometimes the same value for both qc1 and qc2. Therefore, we explicitly set different values for `SignerIndices`.
	qc1 := unittest.QuorumCertificateFixture(unittest.QCWithSignerIndices([]byte{85, 0}))
	qc2 := unittest.QuorumCertificateFixture(unittest.QCWithSignerIndices([]byte{90, 0}))
	require.False(t, qc1.Equals(qc2), "Initially, all fields are different, so the objects should not be equal")

	// List of mutations to apply on qc1 to gradually make it equal to qc2
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

// TestNewQuorumCertificate verifies the behavior of the NewQuorumCertificate constructor.
// Test Cases:
//
// 1. Valid input:
//   - Ensures a QuorumCertificate is returned when all fields are populated.
//
// 2. Missing BlockID:
//   - Ensures an error is returned when BlockID is ZeroID.
//
// 3. Nil SignerIndices:
//   - Ensures an error is returned when SignerIndices is nil.
//
// 4. Empty SignerIndices slice:
//   - Ensures an error is returned when SignerIndices is empty.
//
// 5. Nil SigData:
//   - Ensures an error is returned when SigData is nil.
//
// 6. Empty SigData slice:
//   - Ensures an error is returned when SigData is empty.
func TestNewQuorumCertificate(t *testing.T) {
	view := uint64(10)
	blockID := unittest.IdentifierFixture()
	signerIndices := []byte{0x01, 0x02}
	sigData := []byte{0x03, 0x04}

	base := flow.UntrustedQuorumCertificate{
		View:          view,
		BlockID:       blockID,
		SignerIndices: signerIndices,
		SigData:       sigData,
	}

	t.Run("valid input", func(t *testing.T) {
		qc, err := flow.NewQuorumCertificate(base)
		assert.NoError(t, err)
		assert.NotNil(t, qc)
		assert.Equal(t, view, qc.View)
		assert.Equal(t, blockID, qc.BlockID)
		assert.Equal(t, signerIndices, qc.SignerIndices)
		assert.Equal(t, sigData, qc.SigData)
	})

	t.Run("missing BlockID", func(t *testing.T) {
		u := base
		u.BlockID = flow.ZeroID

		qc, err := flow.NewQuorumCertificate(u)
		assert.Error(t, err)
		assert.Nil(t, qc)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("nil SignerIndices", func(t *testing.T) {
		u := base
		u.SignerIndices = nil

		qc, err := flow.NewQuorumCertificate(u)
		assert.Error(t, err)
		assert.Nil(t, qc)
		assert.Contains(t, err.Error(), "SignerIndices")
	})

	t.Run("empty SignerIndices slice", func(t *testing.T) {
		u := base
		u.SignerIndices = []byte{}

		qc, err := flow.NewQuorumCertificate(u)
		assert.Error(t, err)
		assert.Nil(t, qc)
		assert.Contains(t, err.Error(), "SignerIndices")
	})

	t.Run("nil SigData", func(t *testing.T) {
		u := base
		u.SigData = nil

		qc, err := flow.NewQuorumCertificate(u)
		assert.Error(t, err)
		assert.Nil(t, qc)
		assert.Contains(t, err.Error(), "SigData")
	})

	t.Run("empty SigData slice", func(t *testing.T) {
		u := base
		u.SigData = []byte{}

		qc, err := flow.NewQuorumCertificate(u)
		assert.Error(t, err)
		assert.Nil(t, qc)
		assert.Contains(t, err.Error(), "SigData")
	})
}
