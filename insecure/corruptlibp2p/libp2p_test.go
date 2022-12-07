package corruptlibp2p_test

import (
	"context"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/onflow/flow-go/network/p2p/utils"
	"testing"
	"time"

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

func TestSpam(t *testing.T) {
	const messagesToSpam = 3
	sporkId := unittest.IdentifierFixture()

	var router *corrupt.GossipSubRouter
	factory := corruptlibp2p.CorruptibleGossipSubFactory(func(r *corrupt.GossipSubRouter) {
		require.NotNil(t, r)
		router = r // save the router at the initialization time of the factory
	})

	spammerNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		internal.WithCorruptGossipSub(factory,
			corruptlibp2p.CorruptibleGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the spammer node
				return nil
			})),
	)

	received := make(chan struct{})

	// keeps track of how many messages victim received from spammer - to know when to stop listening for more messages
	receivedCounter := 0
	var iHaveReceivedCtlMsgs []pb.ControlMessage
	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		internal.WithCorruptGossipSub(factory,
			corruptlibp2p.CorruptibleGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				iHaves := rpc.GetControl().GetIhave()
				if len(iHaves) == 0 {
					// don't inspect control messages with no IHAVE messages
					return nil
				}
				receivedCounter++
				iHaveReceivedCtlMsgs = append(iHaveReceivedCtlMsgs, *rpc.GetControl())

				if receivedCounter == messagesToSpam {
					close(received) // acknowledge victim received all of spammer's messages
				}
				return nil
			})),
	)
	victimPeerId, err := unittest.PeerIDFromFlowID(&victimId)
	require.NoError(t, err)

	victimPeerInfo, err := utils.PeerAddressInfo(victimId)
	require.NoError(t, err)

	// starts nodes
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()

	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{spammerNode, victimNode}, 100*time.Second)
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{spammerNode, victimNode}, cancel, 100*time.Second)

	// connect spammer and victim
	err = spammerNode.AddPeer(ctx, victimPeerInfo)
	require.NoError(t, err)
	connected, err := spammerNode.IsConnected(victimPeerInfo.ID)
	require.NoError(t, err)
	require.True(t, connected)

	// create new spammer
	spammer := corruptlibp2p.NewSpammerGossipSubRouter(router)
	require.NotNil(t, router)

	// prepare to spam - generate IHAVE control messages
	iHaveSentCtlMsgs := spammer.GenerateIHaveCtlMessages(t, messagesToSpam, 5)

	// start spamming the victim peer
	spammer.SpamIHave(victimPeerId, iHaveSentCtlMsgs)

	// check that victim received spammer's message
	select {
	case <-received:
		return
	case <-time.After(2 * time.Second):
		require.Fail(t, "did not receive spam message")
	}

	// check contents of received messages should match what spammer sent
	require.Equal(t, len(iHaveSentCtlMsgs), len(iHaveReceivedCtlMsgs))
	require.ElementsMatch(t, iHaveReceivedCtlMsgs, iHaveSentCtlMsgs)
}
