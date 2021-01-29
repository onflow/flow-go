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
