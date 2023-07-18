package rpc_inspector

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// StartNodesAndEnsureConnected starts the victim and spammer node and ensures they are both connected.
func startNodesAndEnsureConnected(t *testing.T, ctx irrecoverable.SignalerContext, nodes []p2p.LibP2PNode, sporkID flow.Identifier) {
	p2ptest.StartNodes(t, ctx, nodes, 5*time.Second)
	// prior to the test we should ensure that spammer and victim connect.
	// this is vital as the spammer will circumvent the normal pubsub subscription mechanism and send iHAVE messages directly to the victim.
	// without a prior connection established, directly spamming pubsub messages may cause a race condition in the pubsub implementation.
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
}

func stopNodesAndInspector(t *testing.T, cancel context.CancelFunc, nodes []p2p.LibP2PNode, inspector p2p.GossipSubRPCInspector) {
	p2ptest.StopNodes(t, nodes, cancel, 5*time.Second)
	unittest.RequireComponentsDoneBefore(t, time.Second, inspector)
}
