package libp2p

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
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

	// computing hash of node's identifier
	hasher := hash.NewSHA3_256()
	hash := hasher.ComputeHash([]byte(seed))

	// creates a new random generator based on the hash as a seed
	rng, err := random.NewRand(hash)
	if err != nil {
		return nil, fmt.Errorf("cannot parse hash: %w", err)
	}
	// there is no need to check the error as size>len(idList)>=0
	topListInd, err := rng.SubPermutation(len(idList), size)
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
