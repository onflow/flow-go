// +build timesensitivetest
// This file includes a few time sensitive tests. They might pass on your powerful local machine
// but fail on slow CI machine.
// For now, these tests are only for local. Run it with the build tag on:
// > go test --tags=relic,timesensitivetest ./...

package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	assertLiveness(t, allViews, 60)

	cleanupNodes(nodes)
}

// verify if a node lost some messages, it's still able to catch up.
func TestMessagesLost(t *testing.T) {

	nodes, stopper, hub := createNodes(t, 5, 100, 1000)

	hub.WithFilter(blockNodesForFirstNMessages(50, nodes[0]))
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 60)
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
	assertLiveness(t, allViews, 50)
	cleanupNodes(nodes)
}

// verify if each receiver receive delayed messages, the network can still reach consensus
// the delay might skip some blocks, so should expect to see some gaps in the finalized views
// like this:
// [1 2 3 4 10 11 12 17 20 21 22 23 28 31 33 36 39 44 47 53 58 61 62 79 80 88 89 98 101 106 108 111 115 116 119 120 122 123 124 126 127 128 129 130 133 134 135 138 141 142 143 144]
func TestDelay(t *testing.T) {

	nodes, stopper, hub := createNodes(t, 5, 150, 1500)

	hub.WithFilter(delayReceiverMessagesByRange(hotstuffTimeout/10, hotstuffTimeout/2))
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 60)
	cleanupNodes(nodes)
}

// verify that if a node always
func TestOneNodeBehind(t *testing.T) {
	nodes, stopper, hub := createNodes(t, 5, 150, 1500)

	hub.WithFilter(func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		if receiver == nodes[0] {
			return false, hotstuffTimeout + time.Millisecond
		}
		// no block or delay to other nodes
		return false, 0
	})
	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, 60)
	cleanupNodes(nodes)
}
