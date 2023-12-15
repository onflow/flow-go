package scoring

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubIHaveBrokenPromises_Below_Threshold tests that as long as the spammer stays below the ihave spam thresholds, it is not caught and
// penalized by the victim node.
// The thresholds are:
// Maximum messages that include iHave per heartbeat is: 10 (gossipsub parameter; GossipSubMaxIHaveMessages).
// Threshold for broken promises of iHave per heartbeat is: 10 (Flow-specific) parameter. It means that GossipSub samples one iHave id out of the
// entire RPC and if that iHave id is not eventually delivered within 3 seconds (gossipsub parameter, GossipSubIWantFollowupTime), then the promise is considered broken. We set
// this threshold to 10 meaning that the first 10 broken promises are ignored. This is to allow for some network churn.
// Also, per hearbeat (GossipSubHeartbeatInterval, 1 second ), the spammer is allowed to send at most 5000 ihave messages (gossip sub parameter; GossipSubMaxIHaveLength) on aggregate, and
// excess messages are dropped (without being counted as broken promises).
func TestGossipSubIHaveBrokenPromises_Below_Threshold(t *testing.T) {
	role := flow.RoleConsensus
	sporkId := unittest.IdentifierFixture()
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	receivedIWants := unittest.NewProtectedMap[string, struct{}]()
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	spammer := corruptlibp2p.NewGossipSubRouterSpammerWithRpcInspector(t, sporkId, role, idProvider, func(id peer.ID, rpc *corrupt.RPC) error {
		// override rpc inspector of the spammer node to keep track of the iwants it has received.
		if rpc.RPC.Control == nil || rpc.RPC.Control.Iwant == nil {
			return nil
		}
		for _, iwant := range rpc.RPC.Control.Iwant {
			for _, msgId := range iwant.MessageIDs {
				receivedIWants.Add(msgId, struct{}{})
			}
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// we override some of the default scoring parameters in order to speed up the test in a time-efficient manner.
	blockTopicOverrideParams := scoring.DefaultTopicScoreParams()
	blockTopicOverrideParams.MeshMessageDeliveriesActivation = 1 * time.Second // we start observing the mesh message deliveries after 1 second of the node startup.
	// we disable invalid message delivery parameters, as the way we implement spammer, when it spams ihave messages, it does not sign them. Hence, without decaying the invalid message deliveries,
	// the node would be penalized for invalid message delivery way sooner than it can mount an ihave broken-promises spam attack.
	blockTopicOverrideParams.InvalidMessageDeliveriesWeight = 0.0
	blockTopicOverrideParams.InvalidMessageDeliveriesDecay = 0.0
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithPeerScoreTracerInterval(10*time.Millisecond), // to speed up the test
		p2ptest.EnablePeerScoringWithOverride(&p2p.PeerScoringConfigOverride{
			TopicScoreParams: map[channels.Topic]*pubsub.TopicScoreParams{
				blockTopic: blockTopicOverrideParams,
			},
			DecayInterval: 1 * time.Second, // we override the decay interval to 1 second so that the score is updated within 1 second intervals.
		}),
	)

	ids := flow.IdentityList{&spammer.SpammerId, &victimIdentity}
	idProvider.SetIdentities(ids)
	nodes := []p2p.LibP2PNode{spammer.SpammerNode, victimNode}
	// to suppress the logs of "peer provider has not set"
	victimNode.WithPeersProvider(func() peer.IDSlice {
		return peer.IDSlice{spammer.SpammerNode.ID()}
	})
	spammer.SpammerNode.WithPeersProvider(func() peer.IDSlice {
		return peer.IDSlice{victimNode.ID()}
	})

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	// creates 10 RPCs each with 10 iHave messages, each iHave message has 50 message ids, hence overall, we have 5000 iHave message ids.
	spamIHaveBrokenPromise(t, spammer, blockTopic.String(), receivedIWants, victimNode)

	// wait till victim counts the spam iHaves as broken promises (one per RPC for a total of 10).
	initialBehavioralPenalty := float64(0) // keeps track of the initial behavioral penalty of the spammer node for decay testing.
	require.Eventually(t, func() bool {
		behavioralPenalty, ok := victimNode.PeerScoreExposer().GetBehaviourPenalty(spammer.SpammerNode.ID())
		if !ok {
			return false
		}
		// We set 7 as the threshold to compensate for the scoring decay in between RPC's being processed by the inspector
		// ideally it must be 10 (one per RPC), but we give it a buffer of 3 to account for decays and floating point errors.
		if behavioralPenalty < 7 {
			t.Logf("pending on behavioral penalty %f", behavioralPenalty)
			return false
		}
		t.Logf("success on behavioral penalty %f", behavioralPenalty)
		initialBehavioralPenalty = behavioralPenalty
		return true
		// Note: we have to wait at least 3 seconds for an iHave to be considered as broken promise (gossipsub parameters), we set it to 10
		// seconds to be on the safe side.
		// Also, the internal heartbeat of GossipSub is 1 second, hence, there is no need to have ticks shorter than 1 second.
	}, 10*time.Second, 1*time.Second)

	spammerScore, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
	require.True(t, ok, "sanity check failed, we should have a score for the spammer node")
	// since spammer is not yet considered to be penalized, its score must be greater than the gossipsub health thresholds.
	require.Greaterf(t,
		spammerScore,
		scoring.DefaultGossipThreshold,
		"sanity check failed, the score of the spammer node must be greater than gossip threshold: %f, actual: %f",
		scoring.DefaultGossipThreshold,
		spammerScore)
	require.Greaterf(t,
		spammerScore,
		scoring.DefaultPublishThreshold,
		"sanity check failed, the score of the spammer node must be greater than publish threshold: %f, actual: %f",
		scoring.DefaultPublishThreshold,
		spammerScore)
	require.Greaterf(t,
		spammerScore,
		scoring.DefaultGraylistThreshold,
		"sanity check failed, the score of the spammer node must be greater than graylist threshold: %f, actual: %f",
		scoring.DefaultGraylistThreshold,
		spammerScore)

	// eventually, after a heartbeat the spammer behavioral counter must be decayed
	require.Eventually(t, func() bool {
		behavioralPenalty, ok := victimNode.PeerScoreExposer().GetBehaviourPenalty(spammer.SpammerNode.ID())
		if !ok {
			return false
		}
		if behavioralPenalty >= initialBehavioralPenalty { // after a heartbeat the spammer behavioral counter must be decayed.
			return false
		}

		return true
	}, 2*time.Second, 100*time.Millisecond, "sanity check failed, the spammer behavioral counter must be decayed after a heartbeat")

	// since spammer stays below the threshold, it should be able to exchange messages with the victim node over pubsub.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
}

// TestGossipSubIHaveBrokenPromises_Above_Threshold tests that a continuous stream of spam iHave broken promises will
// eventually cause the spammer node to be graylisted (i.e., no incoming RPCs from the spammer node will be accepted, and
// no outgoing RPCs to the spammer node will be sent).
// The test performs 3 rounds of attacks: each round with 10 RPCs, each RPC with 10 iHave messages, each iHave message with 50 message ids, hence overall, we have 5000 iHave message ids.
// Note that based on GossipSub parameters 5000 iHave is the most one can send within one decay interval.
// First round of attack makes spammers broken promises still below the threshold of 10 RPCs (broken promises are counted per RPC), hence no degradation of the spammers score.
// Second round of attack makes spammers broken promises above the threshold of 10 RPCs, hence a degradation of the spammers score.
// Third round of attack makes spammers broken promises to around 20 RPCs above the threshold, which causes the graylisting of the spammer node.
func TestGossipSubIHaveBrokenPromises_Above_Threshold(t *testing.T) {
	role := flow.RoleConsensus
	sporkId := unittest.IdentifierFixture()
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	receivedIWants := unittest.NewProtectedMap[string, struct{}]()
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	spammer := corruptlibp2p.NewGossipSubRouterSpammerWithRpcInspector(t, sporkId, role, idProvider, func(id peer.ID, rpc *corrupt.RPC) error {
		// override rpc inspector of the spammer node to keep track of the iwants it has received.
		if rpc.RPC.Control == nil || rpc.RPC.Control.Iwant == nil {
			return nil
		}
		for _, iwant := range rpc.RPC.Control.Iwant {
			for _, msgId := range iwant.MessageIDs {
				receivedIWants.Add(msgId, struct{}{})
			}
		}
		return nil
	})

	conf, err := config.DefaultConfig()
	require.NoError(t, err)
	// overcompensate for RPC truncation
	conf.NetworkConfig.GossipSubRPCInspectorsConfig.IHaveRPCInspectionConfig.MaxSampleSize = 10000
	conf.NetworkConfig.GossipSubRPCInspectorsConfig.IHaveRPCInspectionConfig.MaxMessageIDSampleSize = 10000
	conf.NetworkConfig.GossipSubRPCInspectorsConfig.IWantRPCInspectionConfig.MaxSampleSize = 10000
	conf.NetworkConfig.GossipSubRPCInspectorsConfig.IWantRPCInspectionConfig.MaxMessageIDSampleSize = 10000

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// we override some of the default scoring parameters in order to speed up the test in a time-efficient manner.
	blockTopicOverrideParams := scoring.DefaultTopicScoreParams()
	blockTopicOverrideParams.MeshMessageDeliveriesActivation = 1 * time.Second // we start observing the mesh message deliveries after 1 second of the node startup.
	// we disable invalid message delivery parameters, as the way we implement spammer, when it spams ihave messages, it does not sign them. Hence, without decaying the invalid message deliveries,
	// the node would be penalized for invalid message delivery way sooner than it can mount an ihave broken-promises spam attack.
	blockTopicOverrideParams.InvalidMessageDeliveriesWeight = 0.0
	blockTopicOverrideParams.InvalidMessageDeliveriesDecay = 0.0
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.OverrideFlowConfig(conf),
		p2ptest.WithRole(role),
		p2ptest.WithPeerScoreTracerInterval(10*time.Millisecond), // to speed up the test
		p2ptest.EnablePeerScoringWithOverride(&p2p.PeerScoringConfigOverride{
			TopicScoreParams: map[channels.Topic]*pubsub.TopicScoreParams{
				blockTopic: blockTopicOverrideParams,
			},
			DecayInterval: 1 * time.Second, // we override the decay interval to 1 second so that the score is updated within 1 second intervals.
		}),
	)

	ids := flow.IdentityList{&spammer.SpammerId, &victimIdentity}
	idProvider.SetIdentities(ids)
	nodes := []p2p.LibP2PNode{spammer.SpammerNode, victimNode}
	// to suppress the logs of "peer provider has not set"
	victimNode.WithPeersProvider(func() peer.IDSlice {
		return peer.IDSlice{spammer.SpammerNode.ID()}
	})
	spammer.SpammerNode.WithPeersProvider(func() peer.IDSlice {
		return peer.IDSlice{victimNode.ID()}
	})

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	initScore, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
	require.True(t, ok, "score for spammer node must be present")

	// FIRST ROUND OF ATTACK: spammer sends 10 RPCs to the victim node, each containing 500 iHave messages.
	spamIHaveBrokenPromise(t, spammer, blockTopic.String(), receivedIWants, victimNode)
	t.Log("first round of attack finished")

	// wait till victim counts the spam iHaves as broken promises for the second round of attack (one per RPC for a total of 10).
	require.Eventually(t, func() bool {
		behavioralPenalty, ok := victimNode.PeerScoreExposer().GetBehaviourPenalty(spammer.SpammerNode.ID())
		if !ok {
			return false
		}
		// We set 7.5 as the threshold to compensate for the scoring decay in between RPC's being processed by the inspector
		// ideally it must be 10 (one per RPC), but we give it a buffer of 1 to account for decays and floating point errors.
		// note that we intentionally override the decay speed to be 60-times faster in this test.
		if behavioralPenalty < 7.5 {
			t.Logf("[first round] pending on behavioral penalty %f", behavioralPenalty)
			return false
		}

		t.Logf("[first round] success on behavioral penalty %f", behavioralPenalty)
		return true
		// Note: we have to wait at least 3 seconds for an iHave to be considered as broken promise (gossipsub parameters), we set it to 10
		// seconds to be on the safe side.
	}, 10*time.Second, 1*time.Second)

	scoreAfterFirstRound, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
	require.True(t, ok, "score for spammer node must be present")
	// spammer score after first round must not be decreased severely, we account for 10% drop due to under-performing
	// (on sending fresh new messages since that is not part of the test).
	require.Greater(t, scoreAfterFirstRound, 0.9*initScore)

	// SECOND ROUND OF ATTACK: spammer sends 10 RPCs to the victim node, each containing 500 iHave messages.
	spamIHaveBrokenPromise(t, spammer, blockTopic.String(), receivedIWants, victimNode)
	t.Log("second round of attack finished")
	// wait till victim counts the spam iHaves as broken promises for the second round of attack (one per RPC for a total of 10).
	require.Eventually(t, func() bool {
		behavioralPenalty, ok := victimNode.PeerScoreExposer().GetBehaviourPenalty(spammer.SpammerNode.ID())
		if !ok {
			return false
		}

		// ideally we should have 20 (10 from the first round, 10 from the second round), but we give it a buffer of 5 to account for decays and floating point errors.
		// note that we intentionally override the decay speed to be 60-times faster in this test.
		if behavioralPenalty < 15 {
			t.Logf("[second round] pending on behavioral penalty %f", behavioralPenalty)
			return false
		}

		t.Logf("[second round] success on behavioral penalty %f", behavioralPenalty)
		return true
		// Note: we have to wait at least 3 seconds for an iHave to be considered as broken promise (gossipsub parameters), we set it to 10
		// seconds to be on the safe side.
	}, 10*time.Second, 1*time.Second)

	spammerScore, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
	require.True(t, ok, "sanity check failed, we should have a score for the spammer node")
	// with the second round of the attack, the spammer is about 10 broken promises above the threshold (total ~20 broken promises, but the first 10 are not counted).
	// we expect the score to be dropped to initScore - 10 * 10 * 0.01 * scoring.MaxAppSpecificReward, however, instead of 10, we consider 5 about the threshold, to account for decays.
	require.LessOrEqual(t,
		spammerScore,
		initScore-5*5*0.01*scoring.MaxAppSpecificReward,
		"sanity check failed, the score of the spammer node must be less than the initial score minus 8 * 8 * 0.01 * scoring.MaxAppSpecificReward: %f, actual: %f",
		initScore-5*5*0.1*scoring.MaxAppSpecificReward,
		spammerScore)
	require.Greaterf(t,
		spammerScore,
		scoring.DefaultGossipThreshold,
		"sanity check failed, the score of the spammer node must be greater than gossip threshold: %f, actual: %f",
		scoring.DefaultGossipThreshold,
		spammerScore)
	require.Greaterf(t,
		spammerScore,
		scoring.DefaultPublishThreshold,
		"sanity check failed, the score of the spammer node must be greater than publish threshold: %f, actual: %f",
		scoring.DefaultPublishThreshold,
		spammerScore)
	require.Greaterf(t,
		spammerScore,
		scoring.DefaultGraylistThreshold,
		"sanity check failed, the score of the spammer node must be greater than graylist threshold: %f, actual: %f",
		scoring.DefaultGraylistThreshold,
		spammerScore)

	// since the spammer score is above the gossip, graylist and publish thresholds, it should be still able to exchange messages with victim.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	// THIRD ROUND OF ATTACK: spammer sends 10 RPCs to the victim node, each containing 500 iHave messages, we expect spammer to be graylisted.
	spamIHaveBrokenPromise(t, spammer, blockTopic.String(), receivedIWants, victimNode)
	t.Log("third round of attack finished")
	// wait till victim counts the spam iHaves as broken promises for the third round of attack (one per RPC for a total of 10).
	require.Eventually(t, func() bool {
		behavioralPenalty, ok := victimNode.PeerScoreExposer().GetBehaviourPenalty(spammer.SpammerNode.ID())
		if !ok {
			return false
		}
		// ideally we should have 30 (10 from the first round, 10 from the second round, 10 from the third round), but we give it a buffer of 5 to account for decays and floating point errors.
		// note that we intentionally override the decay speed to be 60-times faster in this test.
		if behavioralPenalty < 25 {
			t.Logf("[third round] pending on behavioral penalty %f", behavioralPenalty)
			return false
		}

		t.Logf("[third round] success on behavioral penalty %f", behavioralPenalty)
		return true
		// Note: we have to wait at least 3 seconds for an iHave to be considered as broken promise (gossipsub parameters), we set it to 10
		// seconds to be on the safe side.
	}, 10*time.Second, 1*time.Second)

	spammerScore, ok = victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
	require.True(t, ok, "sanity check failed, we should have a score for the spammer node")
	// with the third round of the attack, the spammer is about 20 broken promises above the threshold (total ~30 broken promises), hence its overall score must be below the gossip, publish, and graylist thresholds, meaning that
	// victim will not exchange messages with it anymore, and also that it will be graylisted meaning all incoming and outgoing RPCs to and from the spammer will be dropped by the victim.
	require.Lessf(t,
		spammerScore,
		scoring.DefaultGossipThreshold,
		"sanity check failed, the score of the spammer node must be less than gossip threshold: %f, actual: %f",
		scoring.DefaultGossipThreshold,
		spammerScore)
	require.Lessf(t,
		spammerScore,
		scoring.DefaultPublishThreshold,
		"sanity check failed, the score of the spammer node must be less than publish threshold: %f, actual: %f",
		scoring.DefaultPublishThreshold,
		spammerScore)
	require.Lessf(t,
		spammerScore,
		scoring.DefaultGraylistThreshold,
		"sanity check failed, the score of the spammer node must be less than graylist threshold: %f, actual: %f",
		scoring.DefaultGraylistThreshold,
		spammerScore)

	// since the spammer score is below the gossip, graylist and publish thresholds, it should not be able to exchange messages with victim anymore.
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(
		t,
		ctx,
		[]p2p.LibP2PNode{spammer.SpammerNode},
		flow.IdentifierList{spammer.SpammerId.NodeID},
		[]p2p.LibP2PNode{victimNode},
		flow.IdentifierList{victimIdentity.NodeID},
		blockTopic,
		1,
		func() interface{} {
			return unittest.ProposalFixture()
		})
}

// spamIHaveBrokenPromises is a test utility function that is exclusive for the TestGossipSubIHaveBrokenPromises_.* tests.
// It creates and sends 10 RPCs each with 1 iHave message, each iHave message has 500 message ids, hence overall, we have 5000 iHave message ids.
// It then sends those iHave spams to the victim node and waits till the victim node responds with iWants for all the spam iHaves.
// There are some notes to consider:
// - we can't send more than one iHave message per RPC in this test, as each iHave should have a distinct topic, and we only have one subscribed topic in the TestGossipSubIHaveBrokenPromises_.* tests.
// - we can't send more than 10 RPCs containing iHave messages per heartbeat (1 sec). This is a gossipsub parameter (GossipSubMaxIHaveMessages). Hence, we choose 10 RPCs to always stay at the threshold.
// - we can't send more than 5000 iHave messages per heartbeat (1 sec). This is a gossipsub parameter (GossipSubMaxIHaveLength). Hence, we choose 500 message ids per iHave message to always stay at the threshold (10 * 500 = 5000).
// - Note that victim nodes picks one iHave id out of the entire RPC and if that iHave id is not eventually delivered within 3 seconds (gossipsub parameter, GossipSubIWantFollowupTime), then the promise is considered broken. Hence, broken promises are counted per RPC (not per iHave message).
// Args:
// - t: the test instance.
// - spammer: the spammer node.
// - topic: the topic to spam.
// - receivedIWants: a map to keep track of the iWants received by the victim node (exclusive to TestGossipSubIHaveBrokenPromises).
// - victimNode: the victim node.
func spamIHaveBrokenPromise(t *testing.T,
	spammer *corruptlibp2p.GossipSubRouterSpammer,
	topic string,
	receivedIWants *unittest.ProtectedMap[string, struct{}],
	victimNode p2p.LibP2PNode) {
	rpcCount := 10
	// we can't send more than one iHave per RPC in this test, as each iHave should have a distinct topic, and we only have one subscribed topic.
	// when the node does not have a topic subscription, it will discard the iHave message.
	iHavesPerRPC := 1
	// there is a cap on the max iHaves a gossipsub node processes per heartbeat (1 sec), we don't want to exceed that (currently 5000 iHave messages per heartbeat).
	messageIdsPerIHave := 500
	spamCtrlMsgs := spammer.GenerateCtlMessages(rpcCount, p2ptest.WithIHave(iHavesPerRPC, messageIdsPerIHave, topic))

	// sanity check
	require.Len(t, spamCtrlMsgs, rpcCount)
	var sentIHaves []string

	// checks that iHave message ids are not duplicated
	for _, msg := range spamCtrlMsgs {
		// sanity check
		require.Len(t, msg.Ihave, iHavesPerRPC)
		for _, iHave := range msg.Ihave {
			// sanity check
			require.Len(t, iHave.MessageIDs, messageIdsPerIHave)
			for _, msgId := range iHave.MessageIDs {
				require.NotContains(t, sentIHaves, msgId)
				sentIHaves = append(sentIHaves, msgId)
			}
		}
	}

	// spams the victim node with spam iHave messages, since iHave messages are for junk message ids, there will be no
	// reply from spammer to victim over the iWants. Hence, the victim must count this towards 10 broken promises eventually.
	// Note that victim nodes picks one iHave id out of the entire RPC and if that iHave id is not eventually delivered within 3 seconds (gossipsub parameter, GossipSubIWantFollowupTime),
	// then the promise is considered broken. Hence, broken promises are counted per RPC (not per iHave message).
	// This sums up to 10 broken promises (1 per RPC).
	wg := sync.WaitGroup{}
	for i := 0; i < len(spamCtrlMsgs); i++ {
		wg.Add(1)
		i := i // capture the loop variable
		go func() {
			defer wg.Done()
			spammer.SpamControlMessage(t, victimNode, []pb.ControlMessage{spamCtrlMsgs[i]})
		}()
		// we wait 100 milliseconds between each RPC to add an artificial delay between RPCs; this is to reduce the chance that all RPCs arrive in the same heartbeat, hence
		// victim node dropping some.
		time.Sleep(100 * time.Millisecond)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "sanity check failed, we should have sent all the spam iHaves to the victim node")

	// wait till all the spam iHaves are responded with iWants.
	require.Eventually(t,
		func() bool {
			for _, msgId := range sentIHaves {
				if _, ok := receivedIWants.Get(msgId); !ok {
					return false
				}
			}

			return true
		}, 10*time.Second,
		100*time.Millisecond,
		fmt.Sprintf("sanity check failed, we should have received all the iWants for the spam iHaves, expected: %d, actual: %d", len(sentIHaves), receivedIWants.Size()))
}
