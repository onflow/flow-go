package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// SigAggregator aggregates signatures into an aggregated signature
type SigAggregator interface {
	Aggregate([]*hotstuff.SingleSignature) (*hotstuff.AggregatedSignature, error)
}
