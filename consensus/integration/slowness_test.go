package integration_test

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func runNodes(nodes []*Node) {
	for _, n := range nodes {
		go func(n *Node) {
			n.compliance.Ready()
		}(n)
	}
}

func estSlowdown(t *testing.T) {

	// create a network hub for all nodes
	nodes, stopper, hub := createNodes(t, 10, 250000, 200000)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 90)

	cleanupNodes(nodes)
}

// happy path: with 3 nodes, they can reach consensus
func Test3Nodes(t *testing.T) {

	nodes, stopper, hub := createNodes(t, 3, 100, 1000)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 90)

	cleanupNodes(nodes)
}

// with 5 nodes, and one node completely blocked, the other 4 nodes can still reach consensus
func Test5Nodes(t *testing.T) {

	nodes, stopper, hub := createNodes(t, 5, 100, 1000)

	hub.WithFilter(blockNodes(nodes[0]))
	runNodes(nodes)

	<-stopper.stopped

	header, err := nodes[0].state.Final().Head()
	require.NoError(t, err)

	// the first node was blocked, never finalize any block
	require.Equal(t, uint64(0), header.View)

	nodes = nodes[1:]

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 90)

	cleanupNodes(nodes)
}

// verify if a node lost some messages, it's still able to catch up.
func TestMessagesLost(t *testing.T) {

	nodes, stopper, hub := createNodes(t, 5, 100, 1000)

	hub.WithFilter(blockNodesForFirstNMessages(100, nodes[0]))
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 90)
	cleanupNodes(nodes)
}

// verify if each receiver lost 10% messages, the network can still reach consensus
func TestMessagesLostAcrossNetwork(t *testing.T) {

	nodes, stopper, hub := createNodes(t, 5, 150, 1500)

	hub.WithFilter(blockReceiverMessagesByPercentage(10))
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 90)
	cleanupNodes(nodes)
}

// TODO: verify if each receiver lost 50% messages, the network can't reach consensus

func nextDelay(low, high time.Duration) time.Duration {
	return time.Duration(int64(low) + rand.Int63n(int64(high-low)))
}

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

// assert all finalized views must have reached a given view to ensure enough process has been made
func assertLiveness(t *testing.T, allViews [][]uint64, view uint64) {
	// the shortest chain must made enough progress
	shortest := allViews[0]
	require.Greater(t, len(shortest), 0, "no block was finalized")
	highestView := shortest[len(shortest)-1]
	require.Greater(t, highestView, view, "did not finalize enough block")
}

func printState(t *testing.T, nodes []*Node, i int) {
	n := nodes[i]
	headerN, err := n.state.Final().Head()
	require.NoError(t, err)

	n.log.Info().
		Uint64("finalview", headerN.View).
		Uint64("finalheight", headerN.Height).
		Int("proposal", n.blockproposal).
		Int("vote", n.blockvote).
		Int("syncreq", n.syncreq).
		Int("syncresp", n.syncresp).
		Int("rangereq", n.rangereq).
		Int("batchreq", n.batchreq).
		Int("batchresp", n.batchresp).
		Str("finalViews", fmt.Sprintf("%v", chainViews(t, nodes[i]))).Msg("stats")
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
