package topology

import (
	"github.com/onflow/flow-go/model/flow"
)

// GraphSampler is an interface type responsible to sample a connected graph out of a
// identity list. It is a must-have feature that independent executions of this interface on different
// nodes (with different seeds) create a connected graph.
type ConnectedGraphSampler interface {
	// SampleConnectedGraph receives two lists: all and shouldHave. It then samples a connected fanout
	// for the caller that includes the shouldHave set. Independent invocations of this method over
	// different nodes, should create a connected graph.
	// Fanout is the set of nodes that this instance should get connected to in order to create a
	// connected graph.
	SampleConnectedGraph(all flow.IdentityList, shouldHave flow.IdentityList) flow.IdentityList
}
