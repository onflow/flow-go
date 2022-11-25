package network

import (
	"github.com/onflow/flow-go/model/flow"
)

// Topology provides a subset of nodes which a given node should directly connect in order to form a connected mesh
// for epidemic message dissemination (e.g., publisher and subscriber model).
type Topology interface {
	// Fanout receives IdentityList of entire network and constructs the fanout IdentityList of the node.
	// A node directly communicates with its fanout IdentityList on epidemic dissemination
	// of the messages (i.e., publish and multicast).
	//
	// Fanout is not concurrency safe. It is responsibility of caller to lock for it (if needed).
	Fanout(ids flow.IdentityList) flow.IdentityList
}
