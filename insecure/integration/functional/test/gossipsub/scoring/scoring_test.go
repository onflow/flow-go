package scoring

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubInvalidMessageDelivery_Integration tests that when a victim peer is spammed with invalid messages from
// a spammer peer, the victim will eventually penalize the spammer and stop receiving messages from them.
// Note: the term integration is used here because it requires integrating all components of the libp2p stack.
func TestGossipSubInvalidMessageDelivery_Integration(t *testing.T) {
	t.Parallel()

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

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// we override the decay interval to 1 second so that the score is updated within 1 second intervals.
	cfg.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 1 * time.Second
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.EnablePeerScoringWithOverride(p2p.PeerScoringConfigNoOverride),
	)

	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()
	ids := flow.IdentityList{&spammer.SpammerId, &victimIdentity}
	nodes := []p2p.LibP2PNode{spammer.SpammerNode, victimNode}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	totalSpamMessages := 20
	for i := 0; i <= totalSpamMessages; i++ {
		spammer.SpamControlMessage(t, victimNode,
			spammer.GenerateCtlMessages(1),
			spamMsgFactory(spammer.SpammerNode.ID(), victimNode.ID(), blockTopic))
	}

	// wait for at most 3 seconds for the victim node to penalize the spammer node.
	// Each heartbeat is 1 second, so 3 heartbeats should be enough to penalize the spammer node.
	// Ideally, we should wait for 1 heartbeat, but the score may not be updated immediately after the heartbeat.
	require.Eventually(t, func() bool {
		spammerScore, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
		if !ok {
			return false
		}
		if spammerScore >= scoring.DefaultGossipThreshold {
			// ensure the score is low enough so that no gossip is routed by victim node to spammer node.
			return false
		}
		if spammerScore >= scoring.DefaultPublishThreshold {
			// ensure the score is low enough so that non of the published messages of the victim node are routed to the spammer node.
			return false
		}
		if spammerScore >= scoring.DefaultGraylistThreshold {
			// ensure the score is low enough so that the victim node does not accept RPC messages from the spammer node.
			return false
		}

		return true
	}, 3*time.Second, 100*time.Millisecond)

	topicsSnapshot, ok := victimNode.PeerScoreExposer().GetTopicScores(spammer.SpammerNode.ID())
	require.True(t, ok)
	require.NotNil(t, topicsSnapshot, "topic scores must not be nil")
	require.NotEmpty(t, topicsSnapshot, "topic scores must not be empty")
	blkTopicSnapshot, ok := topicsSnapshot[blockTopic.String()]
	require.True(t, ok)

	// ensure that the topic snapshot of the spammer contains a record of at least (60%) of the spam messages sent. The 60% is to account for the messages that were
	// delivered before the score was updated, after the spammer is PRUNED, as well as to account for decay.
	require.True(t,
		blkTopicSnapshot.InvalidMessageDeliveries > 0.6*float64(totalSpamMessages),
		"invalid message deliveries must be greater than %f. invalid message deliveries: %f",
		0.9*float64(totalSpamMessages),
		blkTopicSnapshot.InvalidMessageDeliveries)

	p2ptest.EnsureNoPubsubExchangeBetweenGroups(
		t,
		ctx,
		[]p2p.LibP2PNode{victimNode},
		flow.IdentifierList{victimIdentity.NodeID},
		[]p2p.LibP2PNode{spammer.SpammerNode},
		flow.IdentifierList{spammer.SpammerId.NodeID},
		blockTopic,
		1,
		func() interface{} {
			return unittest.ProposalFixture()
		})
}

// TestGossipSubMeshDeliveryScoring_UnderDelivery_SingleTopic tests that when a peer is under-performing in a topic mesh, its score is (slightly) penalized.
func TestGossipSubMeshDeliveryScoring_UnderDelivery_SingleTopic(t *testing.T) {
	t.Parallel()

	role := flow.RoleConsensus
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	// we override some of the default scoring parameters in order to speed up the test in a time-efficient manner.
	blockTopicOverrideParams := scoring.DefaultTopicScoreParams()
	blockTopicOverrideParams.MeshMessageDeliveriesActivation = 1 * time.Second // we start observing the mesh message deliveries after 1 second of the node startup.

	conf, err := config.DefaultConfig()
	require.NoError(t, err)
	// we override the decay interval to 1 second so that the score is updated within 1 second intervals.
	conf.NetworkConfig.GossipSub.ScoringParameters.DecayInterval = 1 * time.Second
	conf.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 1 * time.Second
	thisNode, thisId := p2ptest.NodeFixture( // this node is the one that will be penalizing the under-performer node.
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.OverrideFlowConfig(conf),
		p2ptest.EnablePeerScoringWithOverride(
			&p2p.PeerScoringConfigOverride{
				TopicScoreParams: map[channels.Topic]*pubsub.TopicScoreParams{
					blockTopic: blockTopicOverrideParams,
				},
			}),
	)

	underPerformerNode, underPerformerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
	)

	idProvider.On("ByPeerID", thisNode.ID()).Return(&thisId, true).Maybe()
	idProvider.On("ByPeerID", underPerformerNode.ID()).Return(&underPerformerId, true).Maybe()
	ids := flow.IdentityList{&underPerformerId, &thisId}
	nodes := []p2p.LibP2PNode{underPerformerNode, thisNode}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// initially both nodes should be able to publish and receive messages from each other in the topic mesh.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	// Also initially the under-performing node should have a score that is at least equal to the MaxAppSpecificReward.
	// The reason is in our scoring system, we reward the staked nodes by MaxAppSpecificReward, and the under-performing node is considered staked
	// as it is in the id provider of thisNode.
	require.Eventually(t, func() bool {
		underPerformingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(underPerformerNode.ID())
		if !ok {
			return false
		}
		if underPerformingNodeScore < scoring.MaxAppSpecificReward {
			// ensure the score is high enough so that gossip is routed by victim node to spammer node.
			return false
		}

		return true
	}, 1*time.Second, 100*time.Millisecond)

	// however, after one decay interval, we expect the score of the under-performing node to be penalized by -0.05 * MaxAppSpecificReward as
	// it has not been able to deliver messages to this node in the topic mesh since the past decay interval.
	require.Eventually(t, func() bool {
		underPerformingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(underPerformerNode.ID())
		if !ok {
			return false
		}
		if underPerformingNodeScore > 0.96*scoring.MaxAppSpecificReward { // score must be penalized by -0.05 * MaxAppSpecificReward.
			// 0.96 is to account for floating point errors.
			return false
		}
		if underPerformingNodeScore < scoring.DefaultGossipThreshold { // even the node is slightly penalized, it should still be able to gossip with this node.
			return false
		}
		if underPerformingNodeScore < scoring.DefaultPublishThreshold { // even the node is slightly penalized, it should still be able to publish to this node.
			return false
		}
		if underPerformingNodeScore < scoring.DefaultGraylistThreshold { // even the node is slightly penalized, it should still be able to establish rpc connection with this node.
			return false
		}

		return true
	}, 3*time.Second, 100*time.Millisecond)

	// even though the under-performing node is penalized, it should still be able to publish and receive messages from this node in the topic mesh.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
}

// TestGossipSubMeshDeliveryScoring_UnderDelivery_TwoTopics tests that when a peer is under-performing in two topics, it is penalized in both topics.
func TestGossipSubMeshDeliveryScoring_UnderDelivery_TwoTopics(t *testing.T) {
	t.Parallel()

	role := flow.RoleConsensus
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	dkgTopic := channels.TopicFromChannel(channels.DKGCommittee, sporkId)

	// we override some of the default scoring parameters in order to speed up the test in a time-efficient manner.
	blockTopicOverrideParams := scoring.DefaultTopicScoreParams()
	blockTopicOverrideParams.MeshMessageDeliveriesActivation = 1 * time.Second // we start observing the mesh message deliveries after 1 second of the node startup.
	dkgTopicOverrideParams := scoring.DefaultTopicScoreParams()
	dkgTopicOverrideParams.MeshMessageDeliveriesActivation = 1 * time.Second // we start observing the mesh message deliveries after 1 second of the node startup.

	conf, err := config.DefaultConfig()
	require.NoError(t, err)
	// we override the decay interval to 1 second so that the score is updated within 1 second intervals.
	conf.NetworkConfig.GossipSub.ScoringParameters.DecayInterval = 1 * time.Second
	conf.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 1 * time.Second
	thisNode, thisId := p2ptest.NodeFixture( // this node is the one that will be penalizing the under-performer node.
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.OverrideFlowConfig(conf),
		p2ptest.EnablePeerScoringWithOverride(
			&p2p.PeerScoringConfigOverride{
				TopicScoreParams: map[channels.Topic]*pubsub.TopicScoreParams{
					blockTopic: blockTopicOverrideParams,
					dkgTopic:   dkgTopicOverrideParams,
				},
			}),
	)

	underPerformerNode, underPerformerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
	)

	idProvider.On("ByPeerID", thisNode.ID()).Return(&thisId, true).Maybe()
	idProvider.On("ByPeerID", underPerformerNode.ID()).Return(&underPerformerId, true).Maybe()
	ids := flow.IdentityList{&underPerformerId, &thisId}
	nodes := []p2p.LibP2PNode{underPerformerNode, thisNode}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// subscribe to the topics.
	for _, node := range nodes {
		for _, topic := range []channels.Topic{blockTopic, dkgTopic} {
			_, err := node.Subscribe(topic, validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
			require.NoError(t, err)
		}
	}

	// Initially the under-performing node should have a score that is at least equal to the MaxAppSpecificReward.
	// The reason is in our scoring system, we reward the staked nodes by MaxAppSpecificReward, and the under-performing node is considered staked
	// as it is in the id provider of thisNode.
	require.Eventually(t, func() bool {
		underPerformingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(underPerformerNode.ID())
		if !ok {
			return false
		}
		if underPerformingNodeScore < scoring.MaxAppSpecificReward {
			// ensure the score is high enough so that gossip is routed by victim node to spammer node.
			return false
		}

		return true
	}, 2*time.Second, 100*time.Millisecond)

	// No message delivery happens intentionally, so that the under-performing node is penalized.

	// however, after one decay interval, we expect the score of the under-performing node to be penalized by ~ 2 * -0.05 * MaxAppSpecificReward.
	require.Eventually(t, func() bool {
		underPerformingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(underPerformerNode.ID())
		if !ok {
			return false
		}
		if underPerformingNodeScore > 0.91*scoring.MaxAppSpecificReward { // score must be penalized by ~ 2 * -0.05 * MaxAppSpecificReward.
			// 0.91 is to account for the floating point errors.
			return false
		}
		if underPerformingNodeScore < scoring.DefaultGossipThreshold { // even the node is slightly penalized, it should still be able to gossip with this node.
			return false
		}
		if underPerformingNodeScore < scoring.DefaultPublishThreshold { // even the node is slightly penalized, it should still be able to publish to this node.
			return false
		}
		if underPerformingNodeScore < scoring.DefaultGraylistThreshold { // even the node is slightly penalized, it should still be able to establish rpc connection with this node.
			return false
		}

		return true
	}, 3*time.Second, 100*time.Millisecond)

	// even though the under-performing node is penalized, it should still be able to publish and receive messages from this node in both topic meshes.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, dkgTopic, 1, func() interface{} {
		return unittest.DKGMessageFixture()
	})
}

// TestGossipSubMeshDeliveryScoring_Replay_Will_Not_Counted tests that replayed messages will not be counted towards the mesh message deliveries.
func TestGossipSubMeshDeliveryScoring_Replay_Will_Not_Counted(t *testing.T) {
	t.Parallel()

	role := flow.RoleConsensus
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	// we override some of the default scoring parameters in order to speed up the test in a time-efficient manner.
	conf, err := config.DefaultConfig()
	require.NoError(t, err)
	// we override the decay interval to 1 second so that the score is updated within 1 second intervals.
	conf.NetworkConfig.GossipSub.ScoringParameters.DecayInterval = 1 * time.Second
	conf.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 1 * time.Second
	blockTopicOverrideParams := scoring.DefaultTopicScoreParams()
	blockTopicOverrideParams.MeshMessageDeliveriesActivation = 1 * time.Second // we start observing the mesh message deliveries after 1 second of the node startup.
	thisNode, thisId := p2ptest.NodeFixture(                                   // this node is the one that will be penalizing the under-performer node.
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.OverrideFlowConfig(conf),
		p2ptest.EnablePeerScoringWithOverride(
			&p2p.PeerScoringConfigOverride{
				TopicScoreParams: map[channels.Topic]*pubsub.TopicScoreParams{
					blockTopic: blockTopicOverrideParams,
				},
			}),
	)

	replayingNode, replayingId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
	)

	idProvider.On("ByPeerID", thisNode.ID()).Return(&thisId, true).Maybe()
	idProvider.On("ByPeerID", replayingNode.ID()).Return(&replayingId, true).Maybe()
	ids := flow.IdentityList{&replayingId, &thisId}
	nodes := []p2p.LibP2PNode{replayingNode, thisNode}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// initially both nodes should be able to publish and receive messages from each other in the block topic mesh.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	// Initially the replaying node should have a score that is at least equal to the MaxAppSpecificReward.
	// The reason is in our scoring system, we reward the staked nodes by MaxAppSpecificReward, and initially every node is considered staked
	// as it is in the id provider of thisNode.
	initialReplayingNodeScore := float64(0)
	require.Eventually(t, func() bool {
		replayingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(replayingNode.ID())
		if !ok {
			return false
		}
		if replayingNodeScore < scoring.MaxAppSpecificReward {
			// ensure the score is high enough so that gossip is routed by victim node to spammer node.
			return false
		}

		initialReplayingNodeScore = replayingNodeScore
		return true
	}, 2*time.Second, 100*time.Millisecond)

	// replaying node acts honestly and sends 200 block proposals on the topic mesh. This is twice the
	// defaultTopicMeshMessageDeliveryThreshold, which prevents the replaying node to be penalized.
	proposalList := make([]*messages.BlockProposal, 200)
	for i := 0; i < len(proposalList); i++ {
		proposalList[i] = unittest.ProposalFixture()
	}
	i := -1
	p2ptest.EnsurePubsubMessageExchangeFromNode(t, ctx, replayingNode, thisNode, thisId.NodeID, blockTopic, len(proposalList), func() interface{} {
		i += 1
		return proposalList[i]
	})

	// as the replaying node is not penalized, we expect its score to be equal to the initial score.
	require.Eventually(t, func() bool {
		replayingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(replayingNode.ID())
		if !ok {
			return false
		}
		if replayingNodeScore < scoring.MaxAppSpecificReward {
			// ensure the score is high enough so that gossip is routed by victim node to spammer node.
			return false
		}
		if replayingNodeScore != initialReplayingNodeScore {
			// ensure the score is not penalized.
			return false
		}

		initialReplayingNodeScore = replayingNodeScore
		return true
	}, 2*time.Second, 100*time.Millisecond)

	// now the replaying node acts maliciously and just replays the same messages again.
	i = -1
	p2ptest.EnsureNoPubsubMessageExchange(
		t,
		ctx,
		[]p2p.LibP2PNode{replayingNode},
		[]p2p.LibP2PNode{thisNode},
		flow.IdentifierList{thisId.NodeID},
		blockTopic,
		len(proposalList),
		func() interface{} {
			i += 1
			return proposalList[i]
		})

	// since the last decay interval, the replaying node has not delivered anything new, so its score should be penalized for under-performing.
	require.Eventually(t, func() bool {
		replayingNodeScore, ok := thisNode.PeerScoreExposer().GetScore(replayingNode.ID())
		if !ok {
			return false
		}

		if replayingNodeScore >= initialReplayingNodeScore {
			// node must be penalized for just replaying the same messages.
			return false
		}

		if replayingNodeScore >= scoring.MaxAppSpecificReward {
			// node must be penalized for just replaying the same messages.
			return false
		}

		// following if-statements check that even though the node is penalized, it is not penalized too much, and
		// can still participate in the network. We don't desire to disallow list a node for just under-performing.
		if replayingNodeScore < scoring.DefaultGossipThreshold {
			return false
		}

		if replayingNodeScore < scoring.DefaultPublishThreshold {
			return false
		}

		if replayingNodeScore < scoring.DefaultGraylistThreshold {
			return false
		}

		initialReplayingNodeScore = replayingNodeScore
		return true
	}, 2*time.Second, 100*time.Millisecond)

	// even though the replaying node is penalized, it should still be able to publish and receive messages from this node in both topic meshes.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
}
