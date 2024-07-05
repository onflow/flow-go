package topology

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// CustomTopology returns a custom set of nodes as the fanout.
type CustomTopology struct {
	ids flow.IdentityList
}

var _ network.Topology = &CustomTopology{}

func NewCustomTopology(ids flow.IdentityList) network.Topology {
	return &CustomTopology{
		ids: ids,
	}
}

func (f CustomTopology) Fanout(ids flow.IdentityList) flow.IdentityList {
	return ids.Filter(f.ids.Selector())
}
