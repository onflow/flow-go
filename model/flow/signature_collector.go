package flow

import (
	"github.com/onflow/flow-go/crypto"
)

// SignatureCollector contains a set of of signatures from verifiers attesting
// to the validity of an execution result chunk.
// NOT concurrency safe.
// TODO: this will be replaced with stateful BLS aggregation
type SignatureCollector struct {
	// List of signatures
	verifierSignatures []crypto.Signature
	// List of signer identifiers
	signerIDs []Identifier

	// set of all signerIDs for de-duplicating signatures; the mapped value
	// is the storage index in the verifierSignatures and signerIDs
	signerIDSet map[Identifier]int
}

// NewSignatureCollector instantiates a new SignatureCollector
func NewSignatureCollector() SignatureCollector {
	return SignatureCollector{
		verifierSignatures: nil,
		signerIDs:          nil,
		signerIDSet:        make(map[Identifier]int),
	}
}

// ToAggregatedSignature generates an aggregated signature from all signatures
// in the SignatureCollector
func (c *SignatureCollector) ToAggregatedSignature() AggregatedSignature {
	signatures := make([]crypto.Signature, len(c.verifierSignatures))
	copy(signatures, c.verifierSignatures)

	signers := make([]Identifier, len(c.signerIDs))
	copy(signers, c.signerIDs)

	return AggregatedSignature{
		VerifierSignatures: signatures,
		SignerIDs:          signers,
	}
}

// BySigner returns a signer's signature if it exists
func (c *SignatureCollector) BySigner(signerID Identifier) (*crypto.Signature, bool) {
	idx, found := c.signerIDSet[signerID]
	if !found {
		return nil, false
	}
	return &c.verifierSignatures[idx], true
}

// HasSigned checks if signer has already provided a signature
func (c *SignatureCollector) HasSigned(signerID Identifier) bool {
	_, found := c.signerIDSet[signerID]
	return found
}

// Add appends a signature. Only the _first_ signature is retained for each signerID.
// It returns boolean value to notify if signer was added or not
func (c *SignatureCollector) Add(signerID Identifier, signature crypto.Signature) bool {
	if _, found := c.signerIDSet[signerID]; found {
		return false
	}
	c.signerIDSet[signerID] = len(c.signerIDs)
	c.signerIDs = append(c.signerIDs, signerID)
	c.verifierSignatures = append(c.verifierSignatures, signature)
	return true
}

// NumberSignatures returns the number of stored (distinct) signatures
func (c *SignatureCollector) NumberSignatures() uint {
	return uint(len(c.signerIDs))
}
