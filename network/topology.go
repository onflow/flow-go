package network

import (
	"github.com/onflow/flow-go/model/flow"
)

// Topology provides a subset of nodes which a given node should directly connect to for 1-k messaging.
type Topology interface {
	// GenerateFanout receives IdentityList of entire network and constructs the fanout IdentityList
	// of this instance. A node directly communicates with its fanout IdentityList on epidemic dissemination
	// of the messages (i.e., publish and multicast).
	// Independent invocations of GenerateFanout on different nodes collaboratively must construct a cohesive
	// connected graph of nodes that enables them talking to each other.
	GenerateFanout(ids flow.IdentityList) (flow.IdentityList, error)
}
