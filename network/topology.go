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

// TopologyCache provides caching the most recently generated topology.
// It exposes the same GenerateFanout method as a typical topology interface.
// As long as the input IdentityList to it is the same the cached topology is returned without invoking the
// underlying GenerateFanout.
// This is vital to provide a deterministic topology interface, as by its nature, the Topology interface does not
// guarantee determinesticity.
type TopologyCache interface {
	Topology

	// Get returns the cached fanout of node.
	Get() (flow.IdentityList, bool)

	// FingerPrint returns the finger print of fanout list for which the cache
	// maintains a topology.
	FingerPrint() flow.Identifier

	// Invalidate cleans the cache.
	Invalidate()
}
