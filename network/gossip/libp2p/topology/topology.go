package topology

import "github.com/onflow/flow-go/model/flow"

// Topology provides a subset of nodes which a given node should directly connect to for 1-k messaging
type Topology interface {
	// Subset returns a random subset of the identity list that is passed
	Subset(ids flow.IdentityList, fanout uint, topic string) (flow.IdentityList, error)
}
