package flow_test

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const chunkIdx = uint64(7)

// TestNewAttestation verifies that NewAttestation constructs a valid Attestation
// when given complete, non-zero fields, and returns an error when any required
// field is missing.
// It covers:
//   - valid attestation creation
//   - missing BlockID
//   - missing ExecutionResultID
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

// TestNewResultApprovalBody checks that NewResultApprovalBody builds a valid
// ResultApprovalBody when given a correct Attestation and non-empty
// fields, and returns errors for invalid nested Attestation or missing fields.
// It covers:
//   - valid result approval body creation
//   - invalid nested Attestation
//   - missing ApproverID
//   - missing AttestationSignature
//   - missing Spock proof
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
			ChunkIndex:        chunkIdx,
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

// TestNewResultApproval ensures NewResultApproval combines a valid
// ResultApprovalBody and VerifierSignature into a ResultApproval, and returns
// errors for invalid ResultApprovalBody or missing VerifierSignature.
// It covers:
//   - valid result approval creation
//   - invalid ResultApprovalBody
//   - missing verifier signature
func TestNewResultApproval(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	execResID := unittest.IdentifierFixture()
	approver := unittest.IdentifierFixture()
	attestSig := unittest.SignatureFixture()
	spockSig := unittest.SignatureFixture()
	verifierSig := unittest.SignatureFixture()

	t.Run("valid result approval", func(t *testing.T) {
		att, err := flow.NewAttestation(flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: execResID,
			ChunkIndex:        chunkIdx,
		})
		assert.NoError(t, err)

		rab, err := flow.NewResultApprovalBody(flow.UntrustedResultApprovalBody{
			Attestation:          *att,
			ApproverID:           approver,
			AttestationSignature: attestSig,
			Spock:                spockSig,
		})
		assert.NoError(t, err)

		uv := flow.UntrustedResultApproval{
			Body:              *rab,
			VerifierSignature: verifierSig,
		}

		ra, err := flow.NewResultApproval(uv)
		assert.NoError(t, err)
		assert.NotNil(t, ra)
		assert.Equal(t, *rab, ra.Body)
		assert.Equal(t, verifierSig, ra.VerifierSignature)
	})

	// An invalid ResultApprovalBody must cause NewResultApproval to error
	t.Run("invalid body", func(t *testing.T) {
		uv := flow.UntrustedResultApproval{
			Body: flow.ResultApprovalBody{
				Attestation: flow.Attestation{
					BlockID:           flow.ZeroID,
					ExecutionResultID: execResID,
					ChunkIndex:        chunkIdx,
				},
				ApproverID:           approver,
				AttestationSignature: attestSig,
				Spock:                spockSig,
			},
			VerifierSignature: verifierSig,
		}

		ra, err := flow.NewResultApproval(uv)
		assert.Error(t, err)
		assert.Nil(t, ra)
		assert.Contains(t, err.Error(), "invalid result approval body")
	})

	// Missing VerifierSignature must cause NewResultApproval to error
	t.Run("empty verifier signature", func(t *testing.T) {
		att, err := flow.NewAttestation(flow.UntrustedAttestation{
			BlockID:           blockID,
			ExecutionResultID: execResID,
			ChunkIndex:        3,
		})
		assert.NoError(t, err)

		rab, err := flow.NewResultApprovalBody(flow.UntrustedResultApprovalBody{
			Attestation:          *att,
			ApproverID:           approver,
			AttestationSignature: attestSig,
			Spock:                spockSig,
		})
		assert.NoError(t, err)

		uv := flow.UntrustedResultApproval{
			Body:              *rab,
			VerifierSignature: crypto.Signature{},
		}

		ra, err := flow.NewResultApproval(uv)
		assert.Error(t, err)
		assert.Nil(t, ra)
		assert.Contains(t, err.Error(), "VerifierSignature")
	})
}

func TestResultApprovalEncode(t *testing.T) {
	ra := unittest.ResultApprovalFixture()
	id := ra.ID()
	assert.NotEqual(t, flow.ZeroID, id)
}

func TestResultApprovalBodyNonMalleable(t *testing.T) {
	ra := unittest.ResultApprovalFixture()
	unittest.RequireEntityNonMalleable(t, &ra.Body)
}

// TestResultApprovalNonMalleable confirms that the ResultApproval struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestResultApprovalNonMalleable(t *testing.T) {
	ra := unittest.ResultApprovalFixture()
	unittest.RequireEntityNonMalleable(t, ra)
}

// TestAttestationID_Malleability confirms that the Attestation struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestAttestationID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.AttestationFixture())
}
