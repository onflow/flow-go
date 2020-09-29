package topology

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

var _ Topology = &RandPermTopology{}

// RandPermTopology generates a random topology from a given set of nodes and for a given role
// The topology generated is a union of three sets:
// 1. a random subset of the size (n+1)/2 to make sure the nodes form a connected graph with no islands
// 2. one node of each of the flow role from the remaining ids to make a node can talk to any other type of node
// 3. (n+1)/2 of the nodes of the same role as this node from the remaining ids to make sure that nodes of the same type
// form a connected graph with no islands.
type RandPermTopology struct {
	myRole flow.Role
	seed   int64
}

func NewRandPermTopology(role flow.Role, id flow.Identifier) (RandPermTopology, error) {
	seed, err := seedFromID(id)
	if err != nil {
		return RandPermTopology{}, fmt.Errorf("failed to seed topology: %w", err)
	}
	return RandPermTopology{
		myRole: role,
		seed:   seed,
	}, nil
}

func (r RandPermTopology) Subset(idList flow.IdentityList, fanout uint) (flow.IdentityList, error) {

	if uint(len(idList)) < fanout {
		return nil, fmt.Errorf("cannot sample topology idList %d smaller than desired fanout %d", len(idList), fanout)
	}

	// connect to (n+1)/2 other nodes to ensure graph is connected (no islands)
	result, remainder := connectedGraphSample(idList, r.seed)

	// find one id for each role from the remaining list, if it hasn't already been chosen
	for _, role := range flow.Roles() {

		if len(result.Filter(filter.HasRole(role))) > 0 {
			// we already have a node with this role
			continue
		}

		var selectedIDs flow.IdentityList

		// connect to one of each type (ignore remainder)
		selectedIDs, _ = oneOfEachRoleSample(idList, r.seed, role)

		// add it to result
		result = append(result, selectedIDs...)
	}

	// connect to (k+1)/2 other nodes of the same type to ensure all nodes of the same type are fully connected,
	// where k is the number of nodes of each type
	selfRoleIDs, _ := connectedGraphByRoleSample(remainder, r.seed, r.myRole) // ignore the remaining ids

	result = append(result, selfRoleIDs...)

	return result, nil
}
