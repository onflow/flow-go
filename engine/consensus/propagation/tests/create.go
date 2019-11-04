package tests

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/network/mock"
)

func prepareNRandomNodesAndMRandomCollectionHashes() (
	[]*mockPropagationNode, []*collection.GuaranteedCollection, error) {

	rand.Seed(time.Now().UnixNano())

	N := rand.Intn(100) + 1 // at least 1 node, at most 100 nodes
	M := rand.Intn(200) + 1 // at least 1 collection, at most 200 collections

	// prepare N connected nodes
	entries := make([]string, N)
	for e := 0; e < N; e++ {
		entries[e] = fmt.Sprintf("consensus-consensus%v@localhost:10%v", e, e)
	}
	_, nodes, err := createConnectedNodes(entries...)

	// prepare M distinct collection hashes
	gcs := make([]*collection.GuaranteedCollection, M)
	for m := 0; m < M; m++ {
		gcs[m] = randCollection()
	}
	return nodes, gcs, err
}

// given a list of node entries, return a list of mock nodes and connect them all to a hub
func createConnectedNodes(nodeEntries ...string) (*mock.Hub, []*mockPropagationNode, error) {
	if len(nodeEntries) == 0 {
		return nil, nil, errors.New("NodeEntries must not be empty")
	}

	hub := mock.NewNetworkHub()

	nodes := make([]*mockPropagationNode, 0)
	for i := range nodeEntries {
		node, err := newMockPropagationNode(hub, nodeEntries, i)
		if err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, node)
	}

	return hub, nodes, nil
}

// a utility func to return a random collection hash
func randHash() []byte {
	hash := make([]byte, 32)
	_, err := rand.Read(hash)
	if err != nil {
		panic(err.Error()) // should never error
	}
	return hash
}

// a utiliy func to generate a GuaranteedCollection with random hash
func randCollection() *collection.GuaranteedCollection {
	hash := randHash()
	return &collection.GuaranteedCollection{
		Hash: hash,
	}
}
