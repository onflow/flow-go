package topology

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type LinearFanoutGraphSampler struct {
	seed       int64
	fanoutFunc FanoutFunc
}

// NewLinearFanoutGraphSampler creates and returns a LinearFanoutGraphSampler
func NewLinearFanoutGraphSampler(id flow.Identifier) (*LinearFanoutGraphSampler, error) {
	seed, err := seedFromID(id)
	if err != nil {
		return nil, fmt.Errorf("could not generate seed from id:%w", err)
	}
	return &LinearFanoutGraphSampler{
		seed:       seed,
		fanoutFunc: LinearFanoutFunc,
	}, nil
}

// SampleConnectedGraph receives two lists: all and shouldHave. It then samples a connected fanout
// for the caller that includes the shouldHave set. Independent invocations of this method over
// different nodes, should create a connected graph.
// Fanout is the set of nodes that this instance should get connected to in order to create a
// connected graph.
func (l *LinearFanoutGraphSampler) SampleConnectedGraph(all flow.IdentityList,
	shouldHave flow.IdentityList) flow.IdentityList {
	var result flow.IdentityList
	if shouldHave == nil {
		result = l.connectedGraph(all, l.seed)
	} else {
		result = l.conditionalConnectedGraph(all, shouldHave, l.seed)
	}
	return result
}

// conditionalConnectedGraph returns a random subset of length (n+1)/2, which includes the shouldHave
// set of identifiers.
// If each node connects to the nodes returned by connectedGraph, the graph of such nodes is connected.
func (l *LinearFanoutGraphSampler) conditionalConnectedGraph(all, shouldHave flow.IdentityList, seed int64) flow.IdentityList {
	// total sample size
	totalSize := LinearFanoutFunc(len(all))

	if totalSize < len(shouldHave) {
		// should have set is larger than the required sample for connectivity
		// hence the should have set itself is sufficient to make a connected component.
		return shouldHave
	}

	// subset size excluding should have ones
	subsetSize := totalSize - len(shouldHave)

	// others are all excluding should have ones
	others := all.Filter(filter.Not(filter.In(shouldHave)))
	others = others.DeterministicSample(uint(subsetSize), seed)
	return others.Union(shouldHave)
}

// connectedGraph returns a random subset of length (n+1)/2.
// If each node connects to the nodes returned by connectedGraph, the graph of such nodes is connected.
func (l *LinearFanoutGraphSampler) connectedGraph(ids flow.IdentityList, seed int64) flow.IdentityList {
	// choose (n+1)/2 random nodes so that each node in the graph will have a degree >= (n+1) / 2,
	// guaranteeing a connected graph.
	size := uint(LinearFanoutFunc(len(ids)))
	return ids.DeterministicSample(size, seed)
}
