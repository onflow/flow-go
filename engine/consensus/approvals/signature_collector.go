package approvals

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// SignatureCollector contains a set of of signatures from verifiers attesting
// to the validity of an execution result chunk.
// NOT concurrency safe.
// TODO: this will be replaced with stateful BLS aggregation
type SignatureCollector struct {
	// List of signatures
	verifierSignatures []crypto.Signature
	// List of signer identifiers
	signerIDs []flow.Identifier

	// set of all signerIDs for de-duplicating signatures; the mapped value
	// is the storage index in the verifierSignatures and signerIDs
	signerIDSet map[flow.Identifier]int
}

// NewSignatureCollector instantiates a new SignatureCollector
func NewSignatureCollector() SignatureCollector {
	return SignatureCollector{
		verifierSignatures: nil,
		signerIDs:          nil,
		signerIDSet:        make(map[flow.Identifier]int),
	}
}

// ToAggregatedSignature generates an aggregated signature from all signatures
// in the SignatureCollector
func (c *SignatureCollector) ToAggregatedSignature() flow.AggregatedSignature {
	signatures := make([]crypto.Signature, len(c.verifierSignatures))
	copy(signatures, c.verifierSignatures)

	signers := make([]flow.Identifier, len(c.signerIDs))
	copy(signers, c.signerIDs)

	return flow.AggregatedSignature{
		VerifierSignatures: signatures,
		SignerIDs:          signers,
	}
}

// BySigner returns a signer's signature if it exists
func (c *SignatureCollector) BySigner(signerID flow.Identifier) (*crypto.Signature, bool) {
	idx, found := c.signerIDSet[signerID]
	if !found {
		return nil, false
	}
	return &c.verifierSignatures[idx], true
}

// HasSigned checks if signer has already provided a signature
func (c *SignatureCollector) HasSigned(signerID flow.Identifier) bool {
	_, found := c.signerIDSet[signerID]
	return found
}

// Add appends a signature. Only the _first_ signature is retained for each signerID.
func (c *SignatureCollector) Add(signerID flow.Identifier, signature crypto.Signature) {
	if _, found := c.signerIDSet[signerID]; found {
		return
	}
	c.signerIDSet[signerID] = len(c.signerIDs)
	c.signerIDs = append(c.signerIDs, signerID)
	c.verifierSignatures = append(c.verifierSignatures, signature)
}

// NumberSignatures returns the number of stored (distinct) signatures
func (c *SignatureCollector) NumberSignatures() uint {
	return uint(len(c.signerIDs))
}
