package libp2p

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

var _ middleware.Topology = &RandPermTopology{}

// RandPermTopology generates a deterministic random topology from a given set of nodes and for a given role
// The topology generated is a union of three sets:
// 1. a random subset of the given size
// 2. one node of each of the flow role from the remaining ids
// 3. half of the nodes of the same role as this node from the remaining ids
type RandPermTopology struct {
	myRole flow.Role
}

func NewRandPermTopology(role flow.Role) middleware.Topology {
	return &RandPermTopology{
		myRole: role,
	}
}

func (r RandPermTopology) Subset(idList flow.IdentityList, size int, seed string) (map[flow.Identifier]flow.Identity, error) {

	if len(idList) < size {
		return nil, fmt.Errorf("cannot sample topology idList %d smaller than desired fanout %d", len(idList), size)
	}

	// computing hash of node's identifier
	hasher := hash.NewSHA3_256()
	hash := hasher.ComputeHash([]byte(seed))

	// creates a new random generator based on the hash as a seed
	rng, err := random.NewRand(hash)
	if err != nil {
		return nil, fmt.Errorf("cannot parse hash: %w", err)
	}

	// find a random subset of the given size from the list
	fanoutIDs, err := randomSubset(idList, size, rng)
	if err != nil {
		return nil, fmt.Errorf("cannot sample topology: %w", err)
	}

	remainder := idList.Filter(filter.Not(filter.In(fanoutIDs)))

	// find one id for each role from the remaining list,
	// if it is not already part of fanoutIDs
	oneOfEachRoleIDs := make(flow.IdentityList, 0)
	for _, role := range flow.Roles() {

		if len(fanoutIDs.Filter(filter.HasRole(role))) > 0 {
			// we already have a node with this role
			continue
		}

		ids := remainder.Filter(filter.HasRole(role))
		if len(ids) == 0 {
			// there are no more nodes of this role to choose from
			continue
		}

		// choose 1 out of all the remaining nodes of this role
		selectedID, err := randomSubset(ids, 1, rng)
		if err != nil {
			return nil, fmt.Errorf("cannot sample topology: %w", err)
		}

		oneOfEachRoleIDs = append(oneOfEachRoleIDs, selectedID[0])
	}

	remainder = remainder.Filter(filter.Not(filter.In(oneOfEachRoleIDs)))

	// find a n/2 random subset of nodes of the given role from the remaining list
	ids := remainder.Filter(filter.HasRole(r.myRole))
	sameRoleIDs := (len(ids) + 1) / 2 // rounded up to the closest integer

	selfRoleIDs, err := randomSubset(ids, sameRoleIDs, rng)
	if err != nil {
		return nil, fmt.Errorf("cannot sample topology: %w", err)
	}

	// combine all three subsets
	finalIDs := append(fanoutIDs, oneOfEachRoleIDs...)
	finalIDs = append(finalIDs, selfRoleIDs...)

	// creates a map of all the selected ids
	topMap := make(map[flow.Identifier]flow.Identity)
	for _, id := range finalIDs {
		topMap[id.NodeID] = *id
	}

	return topMap, nil

}

func randomSubset(ids flow.IdentityList, size int, rnd random.Rand) (flow.IdentityList, error) {

	if size == 0 {
		return []flow.IdentityList{}, nil
	}

	if len(ids) < size {
		return ids, nil
	}

	copy := make(flow.IdentityList, 0, len(ids))
	copy = append(copy, ids...)
	err := rnd.Samples(copy, size, func(i int, j int) {
		copy[i], copy[j] = copy[j], copy[i]
	})
	if err != nil {
		return nil, err
	}

	return copy[:size], nil
}
