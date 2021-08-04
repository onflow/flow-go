package signature

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// ThresholdSigAggregator aggregates the threshold signatures
type ThresholdSigAggregator interface {
	// TrustedAdd adds an already verified signature.
	// return (true, nil) means the signature has been added
	// return (false, nil) means the signature is a duplication
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error)
	// Aggregate assumes enough shares have been collected, it aggregates the signatures
	// and return the aggregated signature.
	// if called concurrently, only one threshold will be running the aggregation.
	Aggregate() ([]byte, error)
}
