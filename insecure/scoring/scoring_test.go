package scoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubInvalidMessageDeliveryScoring ensure that the following invalid message infractions decrease a nodes gossipsub score resulting in the connection
// with the malicious node to be pruned.
// - The gossiped message is missing a signature.
// - The gossiped message has a signature but is missing the signer id.
// - The gossiped message is self-origin i.e., a malicious node is bouncing back our gossiped messages to us.
// - The gossiped message has an invalid message. i:e: invalid message signature.
func TestGossipSubInvalidMessageDeliveryScoring(t *testing.T) {
	role := flow.RoleConsensus
	sporkId := unittest.IdentifierFixture()
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkId, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithPeerScoreTracerInterval(1*time.Second),
		p2ptest.WithPeerScoringEnabled(idProvider),
	)
	fmt.Println("spammer.SpammerNode.Host().ID(): ", spammer.SpammerNode.Host().ID(), "spammer.SpammerId: ", spammer.SpammerId.NodeID)
	fmt.Println("victimNode.Host().ID(): ", victimNode.Host().ID(), "victimIdentity: ", victimIdentity.NodeID)

	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()
	ids := flow.IdentityList{&spammer.SpammerId, &victimIdentity}
	nodes := []p2p.LibP2PNode{spammer.SpammerNode, victimNode}

	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 2*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.EnsureConnected(t, ctx, nodes)

	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		return unittest.ProposalFixture(), blockTopic
	})

	for i := 0; i <= 20; i++ {
		spammer.SpamControlMessage(t, victimNode,
			spammer.GenerateCtlMessages(1),
			p2ptest.PubsubMessageFixture(t, p2ptest.WithFrom(spammer.SpammerNode.Host().ID()), p2ptest.WithNoSignature(), p2ptest.WithTopic(blockTopic.String())))
	}

	// wait for 3 heartbeats to ensure the score is updated.
	time.Sleep(3 * time.Second)

	spammerScore, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.Host().ID())
	require.True(t, ok)
	// ensure the score is low enough so that no gossip is routed by victim node to spammer node.
	require.True(t, spammerScore < scoring.DefaultGossipThreshold, "spammerScore: %d", spammerScore)
	// ensure the score is low enough so that non of the published messages of the victim node are routed to the spammer node.
	require.True(t, spammerScore < scoring.DefaultPublishThreshold, "spammerScore: %d", spammerScore)
	// ensure the score is low enough so that the victim node does not accept RPC messages from the spammer node.
	require.True(t, spammerScore < scoring.DefaultGraylistThreshold, "spammerScore: %d", spammerScore)
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{victimNode}, []p2p.LibP2PNode{spammer.SpammerNode}, func() (interface{}, channels.Topic) {
		return unittest.ProposalFixture(), blockTopic
	})
}
