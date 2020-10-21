package topology

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// connectedGraph returns a random subset of length (n+1)/2.
// If each node connects to the nodes returned by connectedGraph, the graph of such nodes is connected.
func connectedGraph(ids flow.IdentityList, seed int64) flow.IdentityList {
	// choose (n+1)/2 random nodes so that each node in the graph will have a degree >= (n+1) / 2,
	// guaranteeing a connected graph.
	size := uint(math.Ceil(float64(len(ids)+1) / 2))
	return ids.DeterministicSample(size, seed)
}

func connectedGraphSample(ids flow.IdentityList, seed int64) (flow.IdentityList, flow.IdentityList) {
	result := connectedGraph(ids, seed)
	remainder := ids.Filter(filter.Not(filter.In(result)))
	return result, remainder
}

// seedFromID generates a int64 seed from a flow.Identifier
func seedFromID(id flow.Identifier) (int64, error) {
	var seed int64
	buf := bytes.NewBuffer(id[:])
	err := binary.Read(buf, binary.LittleEndian, &seed)
	return seed, err
}
