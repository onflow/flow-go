package flow

import (
	"github.com/onflow/flow-go/crypto"
)

// AggregatedSignature contains a set of of signatures from verifiers attesting
// to the validity of an execution result chunk.
// TODO: this will be replaced with BLS aggregation
type AggregatedSignature struct {
	// List of signatures
	VerifierSignatures []crypto.Signature
	// List of signer identifiers
	SignerIDs []Identifier
}

// Len returns the number of signatures in the AggregatedSignature
func (a *AggregatedSignature) Len() int {
	return len(a.VerifierSignatures)
}

// BySigner returns a signer's signature if it exists
func (a *AggregatedSignature) BySigner(signerID Identifier) (*crypto.Signature, bool) {
	for index, id := range a.SignerIDs {
		if id == signerID {
			return &a.VerifierSignatures[index], true
		}
	}
	return nil, false
}
