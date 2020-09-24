package topology

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

// connectedGraph returns a random subset of length (n+1)/2.
// If each node connects to the nodes returned by connectedGraph, the graph of such nodes is connected.
func connectedGraph(ids flow.IdentityList, seed int64) flow.IdentityList {
	// choose (n+1)/2 random nodes so that each node in the graph will have a degree >= (n+1) / 2,
	// guaranteeing a connected graph.
	size := uint(math.Ceil(float64(len(ids)+1) / 2))
	return ids.RandomDeterministicSample(size, seed)
}

// connectedGraph returns a random subset of length (n+1)/2 of the specified role
func connectedGraphByRole(ids flow.IdentityList, seed int64, role flow.Role) flow.IdentityList {
	filteredIds := ids.Filter(filter.HasRole(role))
	if len(filteredIds) == 0 {
		// there are no more nodes of this role to choose from
		return flow.IdentityList{}
	}
	return connectedGraph(filteredIds, seed)
}

// oneOfEachRole returns one random id of the given role
func oneOfEachRole(ids flow.IdentityList, seed int64, role flow.Role) flow.IdentityList {
	filteredIds := ids.Filter(filter.HasRole(role))
	if len(filteredIds) == 0 {
		// there are no more nodes of this role to choose from
		return flow.IdentityList{}
	}

	// choose 1 out of all the remaining nodes of this role
	selectedID := filteredIds.RandomDeterministicSample(1, seed)

	return selectedID
}

func connectedGraphSample(ids flow.IdentityList, seed int64) (flow.IdentityList, flow.IdentityList) {
	result := connectedGraph(ids, seed)
	remainder := ids.Filter(filter.Not(filter.In(result)))
	return result, remainder
}

func connectedGraphByRoleSample(ids flow.IdentityList, seed int64, role flow.Role) (flow.IdentityList, flow.IdentityList) {
	result := connectedGraphByRole(ids, seed, role)
	remainder := ids.Filter(filter.Not(filter.In(result)))
	return result, remainder
}

func oneOfEachRoleSample(ids flow.IdentityList, seed int64, role flow.Role) (flow.IdentityList, flow.IdentityList) {
	result := oneOfEachRole(ids, seed, role)
	remainder := ids.Filter(filter.Not(filter.In(result)))
	return result, remainder
}

// seedFromID generates the seed to be used for the random function
func seedFromID(id flow.Identifier) (int64, error) {
	var seed int64
	buf := bytes.NewBuffer(id[:])
	err := binary.Read(buf, binary.LittleEndian, &seed)
	return seed, err
}
