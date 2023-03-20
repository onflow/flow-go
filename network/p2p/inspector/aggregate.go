package inspector

import (
	"sync"

	"github.com/hashicorp/go-multierror"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

// AggregateRPCInspector gossip sub RPC inspector that combines multiple RPC inspectors into a single inspector. Each
// individual inspector will be invoked synchronously.
type AggregateRPCInspector struct {
	lock       sync.RWMutex
	inspectors []p2p.BasicGossipSubRPCInspector
}

var _ p2p.BasicGossipSubRPCInspector = (*AggregateRPCInspector)(nil)

// NewAggregateRPCInspector returns new aggregate RPC inspector.
func NewAggregateRPCInspector() *AggregateRPCInspector {
	return &AggregateRPCInspector{
		inspectors: make([]p2p.BasicGossipSubRPCInspector, 0),
	}
}

// AddInspector adds a new inspector to the list of inspectors.
func (a *AggregateRPCInspector) AddInspector(inspector p2p.BasicGossipSubRPCInspector) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.inspectors = append(a.inspectors, inspector)
}

// Inspect func with the p2p.BasicGossipSubRPCInspector func signature that will invoke all the configured inspectors.
func (a *AggregateRPCInspector) Inspect(peerID peer.ID, rpc *pubsub.RPC) error {
	a.lock.RLock()
	defer a.lock.RUnlock()
	var errs *multierror.Error
	for _, inspector := range a.inspectors {
		err := inspector.Inspect(peerID, rpc)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}
