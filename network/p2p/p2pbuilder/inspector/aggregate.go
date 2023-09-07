package inspector

import (
	"github.com/hashicorp/go-multierror"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

// AggregateRPCInspector gossip sub RPC inspector that combines multiple RPC inspectors into a single inspector. Each
// individual inspector will be invoked synchronously.
type AggregateRPCInspector struct {
	inspectors []p2p.GossipSubRPCInspector
}

// NewAggregateRPCInspector returns new aggregate RPC inspector.
func NewAggregateRPCInspector(inspectors ...p2p.GossipSubRPCInspector) *AggregateRPCInspector {
	return &AggregateRPCInspector{
		inspectors: inspectors,
	}
}

// Inspect func with the p2p.GossipSubAppSpecificRpcInspector func signature that will invoke all the configured inspectors.
func (a *AggregateRPCInspector) Inspect(peerID peer.ID, rpc *pubsub.RPC) error {
	var errs *multierror.Error
	for _, inspector := range a.inspectors {
		err := inspector.Inspect(peerID, rpc)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs.ErrorOrNil()
}

func (a *AggregateRPCInspector) Inspectors() []p2p.GossipSubRPCInspector {
	return a.inspectors
}
