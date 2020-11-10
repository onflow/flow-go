package topology

import (
	"github.com/onflow/flow-go/model/flow"
)

type Manager interface {
	// MakeTopology receives identity list of entire network and constructs identity list of topology
	// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
	// of the messages (i.e., publish and multicast).
	// Independent invocations of MakeTopology on different nodes collaboratively
	// constructs a connected graph of nodes that enables them talking to each other.
	MakeTopology(ids flow.IdentityList) (flow.IdentityList, error)
}
