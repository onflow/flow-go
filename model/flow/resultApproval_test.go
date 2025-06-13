package flow_test

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const chunkIdx = uint64(7)

func TestNewAttestation(t *testing.T) {

	t.Run("valid attestation", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()
		resultID := unittest.IdentifierFixture()

		ua := flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: resultID,
			ChunkIndex:        chunkIdx,
		}

		at, err := flow.NewAttestation(ua)
		assert.NoError(t, err)
		assert.NotNil(t, at)
		assert.Equal(t, blockID, at.BlockID)
		assert.Equal(t, resultID, at.ExecutionResultID)
		assert.Equal(t, chunkIdx, at.ChunkIndex)
	})

	t.Run("missing BlockID", func(t *testing.T) {
		resultID := unittest.IdentifierFixture()

		ua := flow.UntrustedAttestation{
			BlockID:           flow.ZeroID,
			ExecutionResultID: resultID,
			ChunkIndex:        chunkIdx,
		}

		at, err := flow.NewAttestation(ua)
		assert.Error(t, err)
		assert.Nil(t, at)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("missing ExecutionResultID", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()

		ua := flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: flow.ZeroID,
			ChunkIndex:        chunkIdx,
		}

		at, err := flow.NewAttestation(ua)
		assert.Error(t, err)
		assert.Nil(t, at)
		assert.Contains(t, err.Error(), "ExecutionResultID")
	})
}

func TestNewResultApprovalBody(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	resultID := unittest.IdentifierFixture()
	approver := unittest.IdentifierFixture()
	attestSig := unittest.SignatureFixture()
	spockSig := unittest.SignatureFixture()

	t.Run("valid result approval body", func(t *testing.T) {
		att, err := flow.NewAttestation(flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: resultID,
			ChunkIndex:        chunkIdx,
		})
		assert.NoError(t, err)

		uc := flow.UntrustedResultApprovalBody{
			Attestation:          *att,
			ApproverID:           approver,
			AttestationSignature: attestSig,
			Spock:                spockSig,
		}

		rab, err := flow.NewResultApprovalBody(uc)
		assert.NoError(t, err)
		assert.NotNil(t, rab)
		assert.Equal(t, *att, rab.Attestation)
		assert.Equal(t, approver, rab.ApproverID)
		assert.Equal(t, attestSig, rab.AttestationSignature)
		assert.Equal(t, spockSig, rab.Spock)
	})

	t.Run("invalid attestation", func(t *testing.T) {
		uc := flow.UntrustedResultApprovalBody{
			Attestation: flow.Attestation{
				BlockID:           flow.ZeroID,
				ExecutionResultID: resultID,
				ChunkIndex:        chunkIdx,
			},
			ApproverID:           approver,
			AttestationSignature: attestSig,
			Spock:                spockSig,
		}

		rab, err := flow.NewResultApprovalBody(uc)
		assert.Error(t, err)
		assert.Nil(t, rab)
		assert.Contains(t, err.Error(), "attestation")
	})

	t.Run("empty ApproverID", func(t *testing.T) {
		att, err := flow.NewAttestation(flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: resultID,
			ChunkIndex:        chunkIdx,
		})
		assert.NoError(t, err)

		uc := flow.UntrustedResultApprovalBody{
			Attestation:          *att,
			ApproverID:           flow.ZeroID,
			AttestationSignature: attestSig,
			Spock:                spockSig,
		}

		rab, err := flow.NewResultApprovalBody(uc)
		assert.Error(t, err)
		assert.Nil(t, rab)
		assert.Contains(t, err.Error(), "ApproverID")
	})

	t.Run("empty AttestationSignature", func(t *testing.T) {
		att, err := flow.NewAttestation(flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: resultID,
			ChunkIndex:        chunkIdx,
		})
		assert.NoError(t, err)

		uc := flow.UntrustedResultApprovalBody{
			Attestation:          *att,
			ApproverID:           approver,
			AttestationSignature: crypto.Signature{},
			Spock:                spockSig,
		}

		rab, err := flow.NewResultApprovalBody(uc)
		assert.Error(t, err)
		assert.Nil(t, rab)
		assert.Contains(t, err.Error(), "AttestationSignature")
	})

	t.Run("empty Spock proof", func(t *testing.T) {
		att, err := flow.NewAttestation(flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: resultID,
			ChunkIndex:        4,
		})
		assert.NoError(t, err)

		uc := flow.UntrustedResultApprovalBody{
			Attestation:          *att,
			ApproverID:           approver,
			AttestationSignature: attestSig,
			Spock:                crypto.Signature{},
		}

		rab, err := flow.NewResultApprovalBody(uc)
		assert.Error(t, err)
		assert.Nil(t, rab)
		assert.Contains(t, err.Error(), "Spock")
	})
}

func TestResultApprovalEncode(t *testing.T) {
	ra := unittest.ResultApprovalFixture()
	id := ra.ID()
	assert.NotEqual(t, flow.ZeroID, id)
}
