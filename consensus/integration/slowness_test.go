package integration_test

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

func runNodes(nodes []*Node) {
	for _, n := range nodes {
		go func(n *Node) {
			n.compliance.Ready()
		}(n)
	}
}

// happy path: with 3 nodes, they can reach consensus
func Test3Nodes(t *testing.T) {
	nodes, stopper := createNodes(t, 3, 100)

	connect(nodes, blockProposals())

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
	nodes, stopper := createNodes(t, 5, 100)

	connect(nodes, blockNodes(nodes[0]))

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
	nodes, stopper := createNodes(t, 5, 100)

	connect(nodes, blockNodesForFirstNMessages(100, nodes[0]))

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
	nodes, stopper := createNodes(t, 5, 150)

	// block 10% messages on receiver
	connect(nodes, blockReceiverMessagesByPercentage(10))

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

// verify if messages were delayed, can still reach consensus
func TestMessagesDelayAcrossNetwork(t *testing.T) {
	endBlock := uint64(150)
	nodes, stopper := createNodes(t, 5, endBlock)

	connect(nodes, func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		switch event.(type) {
		case *messages.BlockProposal:
			return false, nextDelay(hotstuffTimeout/10, hotstuffTimeout*2/3)
		case *messages.BlockVote:
			return false, nextDelay(hotstuffTimeout/10, hotstuffTimeout/5)
		// case *messages.SyncRequest:
		// case *messages.SyncResponse:
		// case *messages.RangeRequest:
		// case *messages.BatchRequest:
		// case *messages.BlockResponse:
		default:
			return false, time.Duration(hotstuffTimeout / 10)
		}
	})

	runNodes(nodes)

	<-stopper.stopped

	for i := range nodes {
		printState(t, nodes, i)
	}
	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)
	// should finalize at least 1/3 of blocks
	assertLiveness(t, allViews, endBlock*1/3)
	cleanupNodes(nodes)
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

func TestSomeDelayed(t *testing.T) {
}

func printState(t *testing.T, nodes []*Node, i int) {
	n := nodes[i]
	headerN, err := n.state.Final().Head()
	require.NoError(t, err)
	// fmt.Printf("instance %v view:%v, height: %v,received proposal:%v,vote:%v,syncreq:%v,syncresp:%v,rangereq:%v,batchreq:%v,batchresp:%v\n",
	// 	i, headerN.View, headerN.Height, n.blockproposal, n.blockvote, n.syncreq, n.syncresp, n.rangereq, n.batchreq, n.batchresp)
	log := n.log.With().
		Uint64("finalview", headerN.View).
		Uint64("finalheight", headerN.Height).
		Int("proposal", n.blockproposal).
		Int("vote", n.blockvote).
		Int("syncreq", n.syncreq).
		Int("syncresp", n.syncresp).
		Int("rangereq", n.rangereq).
		Int("batchreq", n.batchreq).
		Int("batchresp", n.batchresp).
		Str("views", fmt.Sprintf("%v", chainViews(t, n))).
		Logger()

	log.Info().Msg("stats")
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

type BlockOrDelayFunc func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration)

func connect(nodes []*Node, blockOrDelay BlockOrDelayFunc) {
	nodeDict := make(map[flow.Identifier]*Node, len(nodes))
	for _, n := range nodes {
		nodeDict[n.id.ID()] = n
	}

	for _, n := range nodes {
		{
			sender := n
			submit := func(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
				for _, targetID := range targetIDs {
					// find receiver
					receiver, found := nodeDict[targetID]
					if !found {
						continue
					}

					blocked, delay := blockOrDelay(channelID, event, sender, receiver)
					if blocked {
						continue
					}

					go receiveWithDelay(channelID, event, sender, receiver, delay)
				}

				return nil
			}
			n.net.WithSubmit(submit)
		}
	}
}

func receiveWithDelay(channelID uint8, event interface{}, sender, receiver *Node, delay time.Duration) {
	// delay the message sending
	if delay > 0 {
		time.Sleep(delay)
	}

	// statics
	receiver.Lock()
	switch event.(type) {
	case *messages.BlockProposal:
		// add some delay to make sure slow nodes can catch up
		time.Sleep(3 * time.Millisecond)
		receiver.blockproposal++
	case *messages.BlockVote:
		receiver.blockvote++
	case *messages.SyncRequest:
		receiver.syncreq++
	case *messages.SyncResponse:
		receiver.syncresp++
	case *messages.RangeRequest:
		receiver.rangereq++
	case *messages.BatchRequest:
		receiver.batchreq++
	case *messages.BlockResponse:
		receiver.batchresp++
	default:
		panic("received message")
	}
	receiver.Unlock()

	// find receiver engine
	receiverEngine := receiver.net.engines[channelID]

	// give it to receiver engine
	receiverEngine.Submit(sender.id.ID(), event)
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		n.db.Close()
		os.RemoveAll(n.dbDir)
	}
}
