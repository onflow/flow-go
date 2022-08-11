package network

import (
	"github.com/onflow/flow-go/model/flow"
)

// Topology provides a subset of nodes which a given node should directly connect to for 1-k messaging.
type Topology interface {
	// GenerateFanout receives IdentityList of entire network, and list of channels the node is subscribing to.
	// It constructs and returns the fanout IdentityList of node.
	// A node directly communicates with its fanout IdentityList on epidemic dissemination
	// of the messages (i.e., publish and multicast).
	// Independent invocations of GenerateFanout on different nodes collaboratively must construct a cohesive
	// connected graph of nodes that enables them talking to each other.
	//
	// As a convention, GenerateFanout is not required to guarantee any deterministic behavior, i.e.,
	// invocations of GenerateFanout with the same input may result in different fanout sets.
	// One may utilize topology Cache to have a more deterministic endpoint on generating fanout.
	//
	// GenerateFanout is not concurrency safe. It is responsibility of caller to lock for it.
	// with the channels argument, it allows the returned topology to be cached, which is necessary for randomized topology.
	GenerateFanout(ids flow.IdentityList, channels ChannelList) (flow.IdentityList, error)
}
