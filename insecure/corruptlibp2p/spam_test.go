package corruptlibp2p_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSpam_IHave sets up a 2 node test between a victim node and a spammer. The spammer sends a few iHAVE control messages
// to the victim node without being subscribed to any of the same topics.
// The test then checks that the victim node received all the messages from the spammer.
func TestSpam_IHave(t *testing.T) {
	const messagesToSpam = 3
	sporkId := unittest.IdentifierFixture()

	gossipsubRouterSpammer := corruptlibp2p.NewGossipSubRouterSpammer2(t, sporkId)

	allSpamIHavesReceived := sync.WaitGroup{}
	allSpamIHavesReceived.Add(messagesToSpam)

	var iHaveReceivedCtlMsgs []pb.ControlMessage
	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				iHaves := rpc.GetControl().GetIhave()
				if len(iHaves) == 0 {
					// don't inspect control messages with no iHAVE messages
					return nil
				}

				iHaveReceivedCtlMsgs = append(iHaveReceivedCtlMsgs, *rpc.GetControl())
				allSpamIHavesReceived.Done() // acknowledge that victim received a message.
				return nil
			})),
	)

	// starts nodes
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()
	nodes := []p2p.LibP2PNode{gossipsubRouterSpammer.SpammerNode, victimNode}
	p2ptest.StartNodes(t, signalerCtx, nodes, 5*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 5*time.Second)

	gossipsubRouterSpammer.WaitUntilInitialized(t)

	// prior to the test we should ensure that spammer and victim connect and discover each other.
	// this is vital as the spammer will circumvent the normal pubsub subscription mechanism and send iHAVE messages directly to the victim.
	// without a priory connection established, directly spamming pubsub messages may cause a race condition in the pubsub implementation.
	p2ptest.EnsureConnected(t, ctx, nodes)
	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, flow.IdentityList{&gossipsubRouterSpammer.SpammerId, &victimId})
	p2ptest.EnsureStreamCreationInBothDirections(t, ctx, nodes)

	// prepare to spam - generate iHAVE control messages
	iHaveSentCtlMsgs := gossipsubRouterSpammer.GenerateIHaveCtlMessages(t, messagesToSpam, 5)

	// start spamming the victim peer
	gossipsubRouterSpammer.SpamIHave(t, victimNode.Host().ID(), iHaveSentCtlMsgs)

	// check that victim received all spam messages
	unittest.RequireReturnsBefore(t, allSpamIHavesReceived.Wait, 1*time.Second, "victim did not receive all spam messages")

	// check contents of received messages should match what spammer sent
	require.ElementsMatch(t, iHaveReceivedCtlMsgs, iHaveSentCtlMsgs)
}
