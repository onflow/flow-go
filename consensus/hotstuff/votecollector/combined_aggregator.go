package votecollector

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// combinedAggregator aggregates signatures from different signers for the same message.
// The instance must be initialized with the message, the required weight to be
// sufficient for aggregation, as well as the weight for each signer.
type combinedAggregator struct {
	message        []byte                     // the message of the signature
	requiredWeight uint64                     // the required weight to be sufficient for aggregation
	weightTable    map[flow.Identifier]uint64 // a map to lookup weight by node ID
	weight         atomic.Uint64
}

// Verify a vote's signature
func (c *combinedAggregator) Verify(signerID flow.Identifier, sig crypto.Signature) (bool, SigType, error) {
	panic("to be implemented")
}

// TrustedAdd adds a verified signature, and returns whether if has collected enough signature shares
func (c *combinedAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature, sigType SigType) (bool, error) {
	panic("to be implemented")
}

// VerifyAndAdd verifies the signature, if the signature is valid, then it will be added
// the first return value is whether the sig is valid
// the second return value is whether the sig is added
// (false, false, nil) means the signature is invalid and is not added to the aggregator
// (true, true, nil) means the signature is valid and added to the aggregator
// (true, false, nil) means the signature is valid and not added to the aggregator
func (c *combinedAggregator) VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, sigType SigType) (bool, bool, error) {
	panic("to be implemented")
}

// HasSufficientWeight returns whether the signature aggregator has stored enough signature shares.
func (c *combinedAggregator) HasSufficientWeight() bool {
	panic("to be implemented")
}

// AggregateSignature assumes enough signature shares have been collected, and returns aggregated signatures
// The first returned aggregated signature is the staking signature;
// The second returned aggregated signature is the threshold signature;
// For consensus cluster, the second return aggregatd signature is the aggregated random beacon sig shares.
// For collection cluster, the second returned aggregatd signature will always be nil.
func (c *combinedAggregator) AggregateSignature() (flow.AggregatedSignature, flow.AggregatedSignature, error) {
	panic("to be implemented")
}
