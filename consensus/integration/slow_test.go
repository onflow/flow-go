package integration_test

import (
	"context"
	"github.com/onflow/flow-go/module/irrecoverable"
	"testing"
)

// verify if a node lost some messages, it's still able to catch up.
func TestMessagesLost(t *testing.T) {

	stopper := NewStopper(50, 0)
	participantsData := createConsensusIdentities(t, 5)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(blockNodesFirstMessages(10, nodes[0]))
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	runNodes(signalerCtx, nodes)

	<-stopper.stopped

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	stopNodes(t, cancel, nodes)
	cleanupNodes(nodes)
}

//// verify if each receiver lost 10% messages, the network can still reach consensus
//func TestMessagesLostAcrossNetwork(t *testing.T) {
//
//	nodes, stopper, hub := createNodes(t, 5, 1500)
//
//	hub.WithFilter(blockReceiverMessagesByPercentage(10))
//	runNodes(nodes)
//
//	<-stopper.stopped
//
//	for i := range nodes {
//		printState(t, nodes, i)
//	}
//	allViews := allFinalizedViews(t, nodes)
//	assertSafety(t, allViews)
//	assertLiveness(t, allViews, 50)
//	cleanupNodes(nodes)
//}
//
//// verify if each receiver receive delayed messages, the network can still reach consensus
//// the delay might skip some blocks, so should expect to see some gaps in the finalized views
//// like this:
//// [1 2 3 4 10 11 12 17 20 21 22 23 28 31 33 36 39 44 47 53 58 61 62 79 80 88 89 98 101 106 108 111 115 116 119 120 122 123 124 126 127 128 129 130 133 134 135 138 141 142 143 144]
//func TestDelay(t *testing.T) {
//
//	nodes, stopper, hub := createNodes(t, 5, 1500)
//
//	hub.WithFilter(delayReceiverMessagesByRange(hotstuffTimeout/10, hotstuffTimeout/2))
//	runNodes(nodes)
//
//	<-stopper.stopped
//
//	for i := range nodes {
//		printState(t, nodes, i)
//	}
//	allViews := allFinalizedViews(t, nodes)
//	assertSafety(t, allViews)
//	assertLiveness(t, allViews, 60)
//	cleanupNodes(nodes)
//}
//
//// verify that if a node always
//func TestOneNodeBehind(t *testing.T) {
//	nodes, stopper, hub := createNodes(t, 5, 1500)
//
//	hub.WithFilter(func(channelID string, event interface{}, sender, receiver *Node) (bool, time.Duration) {
//		if receiver == nodes[0] {
//			return false, hotstuffTimeout + time.Millisecond
//		}
//		// no block or delay to other nodes
//		return false, 0
//	})
//	runNodes(nodes)
//
//	<-stopper.stopped
//
//	for i := range nodes {
//		printState(t, nodes, i)
//	}
//	allViews := allFinalizedViews(t, nodes)
//	assertSafety(t, allViews)
//	assertLiveness(t, allViews, 60)
//	cleanupNodes(nodes)
//}
