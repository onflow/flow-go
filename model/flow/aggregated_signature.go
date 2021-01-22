package flow

import (
	"sync"

	"github.com/onflow/flow-go/crypto"
)

// AggregatedSignature contains a set of of signatures from verifiers attesting
// to the validity of an execution result chunk.
// TODO: this will be replaced with BLS aggregation
type AggregatedSignature struct {
	sync.Mutex
	// List of signatures
	VerifierSignatures []crypto.Signature
	// List of signer identifiers
	SignerIDs []Identifier
}

// Len returns the number of signatures in the AggregatedSignature
func (a *AggregatedSignature) Len() int {
	a.Lock()
	defer a.Unlock()
	return len(a.VerifierSignatures)
}

// BySigner returns a signer's signature if it exists
func (a *AggregatedSignature) BySigner(signerID Identifier) (*crypto.Signature, bool) {
	a.Lock()
	defer a.Unlock()
	for index, id := range a.SignerIDs {
		if id == signerID {
			return &a.VerifierSignatures[index], true
		}
	}
	return nil, false
}

// Add appends a signature.
// ATTENTION: it does not check for duplicates
func (a *AggregatedSignature) Add(signerID Identifier, signature crypto.Signature) {
	a.Lock()
	defer a.Unlock()
	a.SignerIDs = append(a.SignerIDs, signerID)
	a.VerifierSignatures = append(a.VerifierSignatures, signature)
}

// Copy returns a deep copy of the AggregatedSignature
func (a *AggregatedSignature) Copy() AggregatedSignature {
	a.Lock()
	defer a.Unlock()

	signatures := make([]crypto.Signature, len(a.VerifierSignatures))
	copy(signatures, a.VerifierSignatures)

	signers := make([]Identifier, len(a.SignerIDs))
	copy(signers, a.SignerIDs)

	return AggregatedSignature{
		VerifierSignatures: signatures,
		SignerIDs:          signers,
	}
}
