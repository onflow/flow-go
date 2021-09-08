package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// WeightedSignatureAggregator aggregates the signatures
type WeightedSignatureAggregator struct {
}

var _ hotstuff.WeightedSignatureAggregator = &WeightedSignatureAggregator{}

// Verify returns true if and only if the signature is valid.
// It expects that correct type of signature is passed. Only SigTypeStaking is supported
func (s *WeightedSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	panic("to be implemented")
}

// TrustedAdd adds an already verified signature, and look up the weight for the given signer,
// and add it to the total weight, and returns the total weight that have been collected.
// return (1000, nil) means the signature has been added, and 1000 weight has been collected in total.
// return (1000, nil) means the signature is a duplication and 1000 weight has been collected in total.
func (s *WeightedSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (totalWeight uint64, exception error) {
	panic("to be implemented")
}

// TotalWeight returns the total weight presented by the collected sig shares.
func (s *WeightedSignatureAggregator) TotalWeight() uint64 {
	panic("to be implemented")
}

// Aggregate assumes enough shares have been collected, it aggregates the signatures
// and return the aggregated signature.
// if called concurrently, only one thread will be running the aggregation.
func (s *WeightedSignatureAggregator) Aggregate() ([]flow.Identifier, crypto.Signature, error) {
	panic("to be implemented")
}
