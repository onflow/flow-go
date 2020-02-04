package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// SigAggregator provides abstraction of how vote signatures can be aggregated into an aggregated signature
// and how to verify a single signature and aggregated signature
// SigAggregator is a stateful component. It accumulatively builds an aggregated signature.
// The main reason for SigAggregator to be a stateful component is so that HotStuff doesn't need to know
// the implementation detail of threshold, and check whether threshold has met.
type SigAggregator interface {
	// AddSig accumulatively builds an aggregated signature by adding one signature at a time.
	// When the threshold for signatures to be aggregated has reached, an aggregated signature will be returned,
	// otherwise it will return error.
	// AddSig also calls `VerifySingleSig` internally to validate the input signature.
	AddSig(sig *types.VoteSignatureWithPubKey) (*types.AggregatedSignature, error)
}
