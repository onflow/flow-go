package integration_test

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

func runNodes(nodes []*Node) {
	for _, n := range nodes {
		go func(n *Node) {
			n.compliance.Ready()
			n.sync.Ready()
		}(n)
	}
}

// happy path: with 3 nodes, they can reach consensus
func Test3Nodes(t *testing.T) {
	t.Skip("event loop needs an event handler, which will be replaced later in V2")
	stopper := NewStopper(5, 0)
	rootSnapshot := createRootSnapshot(t, 3)
	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// with 5 nodes, and one node completely blocked, the other 4 nodes can still reach consensus
func Test5Nodes(t *testing.T) {
	t.Skip("event loop needs an event handler, which will be replaced later in V2")
	// 4 nodes should be able finalize at least 3 blocks.
	stopper := NewStopper(2, 1)
	rootSnapshot := createRootSnapshot(t, 5)
	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNodes(nodes[0]))
	runNodes(nodes)

	<-stopper.stopped

	header, err := nodes[0].state.Final().Head()
	require.NoError(t, err)

	// the first node was blocked, never finalize any block
	require.Equal(t, uint64(0), header.View)

	allViews := allFinalizedViews(t, nodes[1:])
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// TODO: verify if each receiver lost 50% messages, the network can't reach consensus

func allFinalizedViews(t *testing.T, nodes []*Node) [][]uint64 {
	allViews := make([][]uint64, 0)

	// verify all nodes arrive at the same state
	for _, node := range nodes {
		views := chainViews(t, node)
		allViews = append(allViews, views)
	}

	// sort all Views by chain length
	sort.Slice(allViews, func(i, j int) bool {
		return len(allViews[i]) < len(allViews[j])
	})

	return allViews
}

func assertSafety(t *testing.T, allViews [][]uint64) {
	// find the longest chain of finalized views
	longest := allViews[len(allViews)-1]

	for _, views := range allViews {
		// each view in a chain should match with the longest chain
		for j, view := range views {
			require.Equal(t, longest[j], view, "each view in a chain must match with the view in longest chain at the same height, but didn't")
		}
	}
}

func chainViews(t *testing.T, node *Node) []uint64 {
	views := make([]uint64, 0)

	head, err := node.state.Final().Head()
	require.NoError(t, err)
	for head != nil && head.View > 0 {
		views = append(views, head.View)
		head, err = node.headers.ByBlockID(head.ParentID)
		require.NoError(t, err)
	}

	// reverse all views to start from lower view to higher view

	low2high := make([]uint64, 0)
	for i := len(views) - 1; i >= 0; i-- {
		low2high = append(low2high, views[i])
	}
	return low2high
}

type BlockOrDelayFunc func(channel network.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration)

// block nothing
func blockNothing(channel network.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
	return false, 0
}

// block all messages sent by or received by a list of denied nodes
func blockNodes(denyList ...*Node) BlockOrDelayFunc {
	blackList := make(map[flow.Identifier]*Node, len(denyList))
	for _, n := range denyList {
		blackList[n.id.ID()] = n
	}
	return func(channel network.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block, notBlock := true, false
		if _, ok := blackList[sender.id.ID()]; ok {
			return block, 0
		}
		if _, ok := blackList[receiver.id.ID()]; ok {
			return block, 0
		}
		return notBlock, 0
	}
}
