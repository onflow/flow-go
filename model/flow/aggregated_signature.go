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

// NumberSigners returns the number of signers that contributed to the AggregatedSignature
func (a *AggregatedSignature) NumberSigners() int {
	return len(a.VerifierSignatures)
}

// HasSigner returns true if and only if signer's signature is part of this aggregated signature
func (a *AggregatedSignature) HasSigner(signerID Identifier) bool {
	for _, id := range a.SignerIDs {
		if id == signerID {
			return true
		}
	}
	return false
}
