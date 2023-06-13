package scoring

import (
	"context"
	"math/rand"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

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
	peerScoringCfg := &p2p.PeerScoringConfig{
		TopicScoreParams: scoring.DefaultTopicScoreParams(sporkId),
	}
	spammer1 := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkId, role, idProvider)
	spammer2 := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkId, role, idProvider)
	spammer3 := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkId, role, idProvider)
	spammer4 := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkId, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithPeerScoringEnabled(idProvider),
		p2ptest.WithPeerScoreParamsOption(peerScoringCfg),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer1.SpammerNode.Host().ID()).Return(&spammer1.SpammerId, true).Maybe()
	idProvider.On("ByPeerID", spammer2.SpammerNode.Host().ID()).Return(&spammer2.SpammerId, true).Maybe()
	idProvider.On("ByPeerID", spammer3.SpammerNode.Host().ID()).Return(&spammer3.SpammerId, true).Maybe()
	idProvider.On("ByPeerID", spammer4.SpammerNode.Host().ID()).Return(&spammer4.SpammerId, true).Maybe()
	ids := flow.IdentityList{&spammer1.SpammerId, &spammer2.SpammerId, &spammer3.SpammerId, &spammer4.SpammerId, &victimIdentity}
	nodes := []p2p.LibP2PNode{spammer1.SpammerNode, spammer2.SpammerNode, spammer3.SpammerNode, spammer4.SpammerNode, victimNode}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 2*time.Second)
	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.EnsureConnected(t, ctx, nodes)
	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		return unittest.ProposalFixture(), blockTopic
	})

	topicStr := blockTopic.String()
	missingSignatureMsg := &pb.Message{
		Data:      []byte("some data"),
		Topic:     &topicStr,
		Signature: nil, // signature missing from message
		From:      []byte(spammer1.SpammerNode.Host().ID()),
		Seqno:     []byte{byte(1)},
	}

	for i := 0; i <= 20; i++ {
		b := make([]byte, 100)
		rand.Read(b)
		missingSignatureMsg.Seqno = b
		spammer1.SpamControlMessage(t, victimNode, spammer1.GenerateCtlMessages(1), missingSignatureMsg)
	}
	time.Sleep(3 * time.Second)
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{victimNode}, []p2p.LibP2PNode{spammer1.SpammerNode}, func() (interface{}, channels.Topic) {
		return unittest.ProposalFixture(), blockTopic
	})
}
