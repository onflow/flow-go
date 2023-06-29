package scoring

import (
	"context"
	"testing"
	"time"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
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

// TestGossipSubInvalidMessageDelivery_Integration tests that when a victim peer is spammed with invalid messages from
// a spammer peer, the victim will eventually penalize the spammer and stop receiving messages from them.
func TestGossipSubInvalidMessageDelivery_Integration(t *testing.T) {
	tt := []struct {
		name           string
		spamMsgFactory func(spammerId peer.ID, victimId peer.ID, topic channels.Topic) *pubsub_pb.Message
	}{
		{
			name: "unknown peer, invalid signature",
			spamMsgFactory: func(spammerId peer.ID, _ peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithTopic(topic.String()))
			},
		},
		{
			name: "unknown peer, missing signature",
			spamMsgFactory: func(spammerId peer.ID, _ peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithTopic(topic.String()), p2ptest.WithoutSignature())
			},
		},
		{
			name: "known peer, invalid signature",
			spamMsgFactory: func(spammerId peer.ID, _ peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithFrom(spammerId), p2ptest.WithTopic(topic.String()))
			},
		},
		{
			name: "known peer, missing signature",
			spamMsgFactory: func(spammerId peer.ID, _ peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithFrom(spammerId), p2ptest.WithTopic(topic.String()), p2ptest.WithoutSignature())
			},
		},
		{
			name: "self-origin, invalid signature", // bounce back our own messages
			spamMsgFactory: func(_ peer.ID, victimId peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithFrom(victimId), p2ptest.WithTopic(topic.String()))
			},
		},
		{
			name: "self-origin, no signature", // bounce back our own messages
			spamMsgFactory: func(_ peer.ID, victimId peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithFrom(victimId), p2ptest.WithTopic(topic.String()), p2ptest.WithoutSignature())
			},
		},
		{
			name: "no sender",
			spamMsgFactory: func(_ peer.ID, victimId peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithoutSignerId(), p2ptest.WithTopic(topic.String()))
			},
		},
		{
			name: "no sender, missing signature",
			spamMsgFactory: func(_ peer.ID, victimId peer.ID, topic channels.Topic) *pubsub_pb.Message {
				return p2ptest.PubsubMessageFixture(t, p2ptest.WithoutSignerId(), p2ptest.WithTopic(topic.String()), p2ptest.WithoutSignature())
			},
		},
		
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			testGossipSubInvalidMessageDeliveryScoring(t, tc.spamMsgFactory)
		})
	}
}

// testGossipSubInvalidMessageDeliveryScoring tests that when a victim peer is spammed with invalid messages from
// a spammer peer, the victim will eventually penalize the spammer and stop receiving messages from them.
// Args:
// - t: the test instance.
// - spamMsgFactory: a function that creates unique invalid messages to spam the victim with.
func testGossipSubInvalidMessageDeliveryScoring(t *testing.T, spamMsgFactory func(peer.ID, peer.ID, channels.Topic) *pubsub_pb.Message) {
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

	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()
	ids := flow.IdentityList{&spammer.SpammerId, &victimIdentity}
	nodes := []p2p.LibP2PNode{spammer.SpammerNode, victimNode}

	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 2*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		return unittest.ProposalFixture(), blockTopic
	})

	totalSpamMessages := 20
	for i := 0; i <= totalSpamMessages; i++ {
		spammer.SpamControlMessage(t, victimNode,
			spammer.GenerateCtlMessages(1),
			spamMsgFactory(spammer.SpammerNode.Host().ID(), victimNode.Host().ID(), blockTopic))
	}

	// wait for 3 heartbeats to ensure the score is updated.
	time.Sleep(3 * time.Second)

	spammerScore, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.Host().ID())
	require.True(t, ok)
	// ensure the score is low enough so that no gossip is routed by victim node to spammer node.
	require.True(t, spammerScore < scoring.DefaultGossipThreshold, "spammer score must be less than gossip threshold. spammerScore: %d, gossip threshold: %d", spammerScore, scoring.DefaultGossipThreshold)
	// ensure the score is low enough so that non of the published messages of the victim node are routed to the spammer node.
	require.True(t, spammerScore < scoring.DefaultPublishThreshold, "spammer score must be less than publish threshold. spammerScore: %d, publish threshold: %d", spammerScore, scoring.DefaultPublishThreshold)
	// ensure the score is low enough so that the victim node does not accept RPC messages from the spammer node.
	require.True(t, spammerScore < scoring.DefaultGraylistThreshold, "spammer score must be less than graylist threshold. spammerScore: %d, graylist threshold: %d", spammerScore, scoring.DefaultGraylistThreshold)

	topicsSnapshot, ok := victimNode.PeerScoreExposer().GetTopicScores(spammer.SpammerNode.Host().ID())
	require.True(t, ok)
	require.NotNil(t, topicsSnapshot, "topic scores must not be nil")
	require.NotEmpty(t, topicsSnapshot, "topic scores must not be empty")
	blkTopicSnapshot, ok := topicsSnapshot[blockTopic.String()]
	require.True(t, ok)

	// ensure that the topic snapshot of the spammer contains a record of at least (60%) of the spam messages sent. The 60% is to account for the messages that were delivered before the score was updated, after the spammer is PRUNED, as well as to account for decay.
	require.True(t, blkTopicSnapshot.InvalidMessageDeliveries > 0.6*float64(totalSpamMessages), "invalid message deliveries must be greater than %f. invalid message deliveries: %f", 0.9*float64(totalSpamMessages), blkTopicSnapshot.InvalidMessageDeliveries)

	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{victimNode}, []p2p.LibP2PNode{spammer.SpammerNode}, func() (interface{}, channels.Topic) {
		return unittest.ProposalFixture(), blockTopic
	})
}
