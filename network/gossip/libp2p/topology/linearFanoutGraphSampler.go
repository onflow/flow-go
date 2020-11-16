package topology

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// LinearFanoutGraphSampler samples a guaranteed connected graph fanout, i.e., independent instances
// of this module with distinct seeds generate a connected graph.
type LinearFanoutGraphSampler struct {
	seed       int64
	fanoutFunc FanoutFunc
}

// NewLinearFanoutGraphSampler creates and returns a LinearFanoutGraphSampler.
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
	shouldHave flow.IdentityList) (flow.IdentityList, error) {
	var result flow.IdentityList

	if len(all) == 0 {
		return nil, fmt.Errorf("empty identity list")
	}

	if shouldHave == nil {
		result = l.connectedGraph(all)
	} else {
		result = l.conditionalConnectedGraph(all, shouldHave)
	}

	return result, nil
}

// conditionalConnectedGraph returns a random subset of length (n+1)/2, which includes the shouldHave
// set of identifiers.
// If each node connects to the nodes returned by connectedGraph, the graph of such nodes is connected.
func (l *LinearFanoutGraphSampler) conditionalConnectedGraph(all, shouldHave flow.IdentityList) flow.IdentityList {
	// total sample size
	totalSize := LinearFanoutFunc(len(all))

	if totalSize < len(shouldHave) {
		// total fanout size needed is already satisfied by shouldHave set.
		return shouldHave, nil
	}

	// subset size excluding should have ones
	subsetSize := totalSize - len(shouldHave)

	// others are all excluding should have ones
	others := all.Filter(filter.Not(filter.In(shouldHave)))
	others = others.DeterministicSample(uint(subsetSize), l.seed)
	return others.Union(shouldHave)
}

// connectedGraph returns a random subset of length (n+1)/2.
// If each node connects to the nodes returned by connectedGraph, the graph of such nodes is connected.
func (l *LinearFanoutGraphSampler) connectedGraph(all flow.IdentityList) flow.IdentityList {
	// choose (n+1)/2 random nodes so that each node in the graph will have a degree >= (n+1) / 2,
	// guaranteeing a connected graph.
	size := uint(LinearFanoutFunc(len(all)))
	return all.DeterministicSample(size, l.seed)
}
