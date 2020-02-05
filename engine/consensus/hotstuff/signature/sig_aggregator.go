package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// SigAggregator is a stateful component to aggregate signatures into aggregated signatures.
// The main reason for it to be stateful is to hide how the threshold is implemented.
// Since BLS signature and threshold signature works differently, hotstuff doesn't need to
// know the implementation detail, but only care about at what point an aggregated signature
// has created.
type SigAggregator interface {
	// AddSig accumulatively builds an aggregated signature by adding one signature at a time.
	// When the threshold for signatures to be aggregated has reached, an aggregated signature
	// will be returned; otherwise an error will be returned.
	// AddSig also calls verifies the signature internally. It returns error if the signature
	// to be added is invalid.
	AddSig(sig *types.VoteSignatureWithPubKey) (*types.AggregatedSignature, error)
}
