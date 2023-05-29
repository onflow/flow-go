package integration_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

func runNodes(signalerCtx irrecoverable.SignalerContext, nodes []*Node) {
	for _, n := range nodes {
		go func(n *Node) {
			n.committee.Start(signalerCtx)
			n.hot.Start(signalerCtx)
			n.voteAggregator.Start(signalerCtx)
			n.timeoutAggregator.Start(signalerCtx)
			n.compliance.Start(signalerCtx)
			n.messageHub.Start(signalerCtx)
			n.sync.Start(signalerCtx)
			<-util.AllReady(n.committee, n.hot, n.voteAggregator, n.timeoutAggregator, n.compliance, n.sync, n.messageHub)
		}(n)
	}
}

func stopNodes(t *testing.T, cancel context.CancelFunc, nodes []*Node) {
	stoppingNodes := make([]<-chan struct{}, 0)
	cancel()
	for _, n := range nodes {
		stoppingNodes = append(stoppingNodes, util.AllDone(
			n.committee,
			n.hot,
			n.voteAggregator,
			n.timeoutAggregator,
			n.compliance,
			n.sync,
			n.messageHub,
		))
	}
	unittest.RequireCloseBefore(t, util.AllClosed(stoppingNodes...), time.Second, "requiring nodes to stop")
}

// happy path: with 3 nodes, they can reach consensus
func Test3Nodes(t *testing.T) {
	stopper := NewStopper(5, 0)
	participantsData := createConsensusIdentities(t, 3)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, runFor := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(blockNothing)

	runFor(30 * time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// with 5 nodes, and one node completely blocked, the other 4 nodes can still reach consensus
func Test5Nodes(t *testing.T) {
	// 4 nodes should be able to finalize at least 3 blocks.
	stopper := NewStopper(2, 1)
	participantsData := createConsensusIdentities(t, 5)
	rootSnapshot := createRootSnapshot(t, participantsData)
	nodes, hub, runFor := createNodes(t, NewConsensusParticipants(participantsData), rootSnapshot, stopper)

	hub.WithFilter(blockNodes(nodes[0]))

	runFor(30 * time.Second)

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

	// reverse all views to runFor from lower view to higher view
	low2high := make([]uint64, 0)
	for i := len(views) - 1; i >= 0; i-- {
		low2high = append(low2high, views[i])
	}
	return low2high
}

// BlockOrDelayFunc is a function for deciding whether a message (or other event) should be
// blocked or delayed. The first return value specifies whether the event should be dropped
// entirely (return value `true`) or should be delivered (return value `false`). The second
// return value specifies the delay by which the message should be delivered.
// Implementations must be CONCURRENCY SAFE.
type BlockOrDelayFunc func(channel channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration)

// blockNothing specifies that _all_ messages should be delivered without delay.
// I.e. this function returns always `false` (no blocking), `0` (no delay).
func blockNothing(_ channels.Channel, _ interface{}, _, _ *Node) (bool, time.Duration) {
	return false, 0
}

// blockNodes specifies that all messages sent or received by any member of the `denyList`
// should be dropped, i.e. we return `true` (block message), `0` (no delay).
// For nodes _not_ in the `denyList`,  we return `false` (no blocking), `0` (no delay).
func blockNodes(denyList ...*Node) BlockOrDelayFunc {
	denyMap := make(map[flow.Identifier]*Node, len(denyList))
	for _, n := range denyList {
		denyMap[n.id.ID()] = n
	}
	// no concurrency protection needed as blackList is only read but not modified
	return func(channel channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		if _, ok := denyMap[sender.id.ID()]; ok {
			return true, 0 // block the message
		}
		if _, ok := denyMap[receiver.id.ID()]; ok {
			return true, 0 // block the message
		}
		return false, 0 // allow the message
	}
}
