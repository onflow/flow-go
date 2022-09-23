package integration_test

import (
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMessagesLost verifies if a node lost some messages, it's still able to catch up.
func TestMessagesLost(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "active-pacemaker, needs some liveness logic to broadcast missing entries")
	stopper := NewStopper(50, 0)
	participantsData := createConsensusIdentities(t, 5)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, start := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(blockNodesFirstMessages(10, nodes[0]))

	start()

	unittest.RequireCloseBefore(t, stopper.stopped, time.Minute, "expect to stop before timeout")

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// TestMessagesLostAcrossNetwork verifies if each receiver lost 10% messages, the network can still reach consensus.
func TestMessagesLostAcrossNetwork(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "active-pacemaker, needs some liveness logic to broadcast missing entries")
	stopper := NewStopper(50, 0)
	participantsData := createConsensusIdentities(t, 5)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, start := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(blockReceiverMessagesRandomly(0.1))

	start()

	unittest.RequireCloseBefore(t, stopper.stopped, time.Minute, "expect to stop before timeout")

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// TestDelay verifies that we still reach consensus even if _all_ messages are significantly
// delayed. Due to the delay, some proposals might be orphaned. The message delay is sampled
// for each message from the interval [0.1*hotstuffTimeout, 0.5*hotstuffTimeout].
func TestDelay(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_LONG_RUNNING, "could run for a while depending on what messages are being dropped")
	stopper := NewStopper(50, 0)
	participantsData := createConsensusIdentities(t, 5)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, start := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(delayReceiverMessagesByRange(hotstuffTimeout/10, hotstuffTimeout/2))

	start()

	unittest.RequireCloseBefore(t, stopper.stopped, time.Minute, "expect to stop before timeout")

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// TestOneNodeBehind verifies that if one node (here node 0) consistently experiences a significant
// delay receiving messages beyond the hotstuff timeout, the committee still can reach consensus.
func TestOneNodeBehind(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "active-pacemaker, on CI channel test doesn't stop, need to revisit stop conditions")
	stopper := NewStopper(50, 0)
	participantsData := createConsensusIdentities(t, 3)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, start := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(func(channelID channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		if receiver == nodes[0] {
			return false, hotstuffTimeout + time.Millisecond
		}
		// no block or delay to other nodes
		return false, 0
	})

	start()

	unittest.RequireCloseBefore(t, stopper.stopped, time.Minute, "expect to stop before timeout")

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// TestTimeoutRebroadcast drops
// * all proposals at view 5
// * the first timeout object per view for _every_ sender
// In this configuration, the _initial_ broadcast is insufficient for replicas to make
// progress in view 5 (neither in the happy path, because the proposal is always dropped
// nor on the unhappy path for the _first_ attempt to broadcast timeout objects). We
// expect that replica will eventually broadcast its timeout object again.
func TestTimeoutRebroadcast(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "active-pacemaker, this test requires rebroadcast of timeout objects")
	stopper := NewStopper(10, 0)
	participantsData := createConsensusIdentities(t, 5)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, start := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	// nodeID -> view -> numTimeoutMessages
	lock := new(sync.Mutex)
	blockedTimeoutObjectsTracker := make(map[flow.Identifier]map[uint64]uint64)
	hub.WithFilter(func(channelID channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		switch m := event.(type) {
		case *messages.BlockProposal:
			return m.Header.View == 5, 0 // drop proposals only for view 5
		case *messages.TimeoutObject:
			// drop first timeout object for every sender for every view
			lock.Lock()
			blockedPerView, found := blockedTimeoutObjectsTracker[sender.id.NodeID]
			if !found {
				blockedPerView = make(map[uint64]uint64)
				blockedTimeoutObjectsTracker[sender.id.NodeID] = blockedPerView
			}
			blocked := blockedPerView[m.View] + 1
			blockedPerView[m.View] = blocked
			lock.Unlock()
			return blocked == 1, 0
		}
		// no block or delay to other nodes
		return false, 0
	})

	start()

	unittest.RequireCloseBefore(t, stopper.stopped, 10*time.Second, "expect to stop before timeout")

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}
