package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

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
