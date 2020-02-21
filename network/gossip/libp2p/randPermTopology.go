package libp2p

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

var _ middleware.Topology = &RandPermTopology{}

// RandPermTopology generates a deterministic random topology from a given set of nodes
type RandPermTopology struct {
}

func NewRandPermTopology() middleware.Topology {
	return &RandPermTopology{}
}

func (r RandPermTopology) Subset(idList flow.IdentityList, size int, seed string) (map[flow.Identifier]flow.Identity, error) {

	// selects subset of the nodes in idMap as large as the size
	if len(idList) < size {
		return nil, fmt.Errorf("cannot sample topology idList %d smaller than desired fanout %d", len(idList), size)
	}

	// step-1: creating hasher
	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return nil, err
	}
	// step-2: computing hash of node's identifier
	hash := hasher.ComputeHash([]byte(seed))
	// step-3: converting hash to an uint64 slice
	s := make([]uint64, 0)
	for _, b := range hash {
		// avoid overflow
		if b == 0 {
			b = b + 1
		}
		s = append(s, uint64(b))
	}

	// creates a new random generator based on the seed
	rng, err := random.NewRand(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse hash: %w", err)
	}
	topListInd, err := random.PermutateSubset(len(idList), size, rng)

	if err != nil {
		return nil, fmt.Errorf("cannot sample topology: %w", err)
	}

	// creates a map of the selected ids
	topMap := make(map[flow.Identifier]flow.Identity)
	for _, v := range topListInd {
		id := idList[v]
		topMap[id.NodeID] = *id
	}

	return topMap, nil
}
