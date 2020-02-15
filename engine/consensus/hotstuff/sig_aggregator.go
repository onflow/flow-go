package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// SigAggregator aggregates signatures into an aggregated signature
type SigAggregator interface {
	Aggregate([]*types.SingleSignature) (*types.AggregatedSignature, error)
}
