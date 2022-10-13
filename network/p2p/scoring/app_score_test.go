package scoring_test

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFullGossipSubConnectivity tests that when the entire network is running by honest nodes,
// pushing access nodes to the edges of the network (i.e., the access nodes are not in the mesh of any honest nodes)
// will not cause the network to partition, i.e., all honest nodes can still communicate with each other through GossipSub.
func TestFullGossipSubConnectivity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mock.NewIdentityProvider(t)

	// two groups of non-access nodes and one group of access nodes.
	groupOneNodes, groupOneIds := p2pfixtures.NodesFixture(t, sporkId, t.Name(), 5,
		p2pfixtures.WithRole(flow.RoleConsensus),
		p2pfixtures.WithPeerScoringEnabled(idProvider))
	groupTwoNodes, groupTwoIds := p2pfixtures.NodesFixture(t, sporkId, t.Name(), 5,
		p2pfixtures.WithRole(flow.RoleCollection),
		p2pfixtures.WithPeerScoringEnabled(idProvider))
	accessNodeGroup, accessNodeIds := p2pfixtures.NodesFixture(t, sporkId, t.Name(), 5,
		p2pfixtures.WithRole(flow.RoleAccess),
		p2pfixtures.WithPeerScoringEnabled(idProvider))

	ids := append(append(groupOneIds, groupTwoIds...), accessNodeIds...)
	nodes := append(append(groupOneNodes, groupTwoNodes...), accessNodeGroup...)

	provider := id.NewFixedIdentityProvider(ids)
	idProvider.On("ByPeerID", mocktestify.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		})
	p2pfixtures.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2pfixtures.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())

	// all nodes subscribe to block topic (common topic among all roles)
	// group one
	groupOneSubs := make([]*pubsub.Subscription, len(groupOneNodes))
	var err error
	for i, node := range groupOneNodes {
		groupOneSubs[i], err = node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
		require.NoError(t, err)
	}
	// group two
	groupTwoSubs := make([]*pubsub.Subscription, len(groupTwoNodes))
	for i, node := range groupTwoNodes {
		groupTwoSubs[i], err = node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
		require.NoError(t, err)
	}
	// access node group
	accessNodeSubs := make([]*pubsub.Subscription, len(accessNodeGroup))
	for i, node := range accessNodeGroup {
		accessNodeSubs[i], err = node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
		require.NoError(t, err)
	}

	// creates a topology as follows:
	// groupOneNodes <--> accessNodeGroup <--> groupTwoNodes
	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, append(groupOneNodes, accessNodeGroup...), append(groupOneIds, accessNodeIds...))
	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, append(groupTwoNodes, accessNodeGroup...), append(groupTwoIds, accessNodeIds...))

	// checks end-to-end message delivery works
	// each node sends a distinct message to all and checks that all nodes receive it.
	for _, node := range nodes {
		proposalMsg := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
		require.NoError(t, node.Publish(ctx, blockTopic, proposalMsg))

		// checks that the message is received by all nodes.
		ctx1s, cancel1s := context.WithTimeout(ctx, 5*time.Second)
		p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, groupOneSubs)
		p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, accessNodeSubs)
		p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, groupTwoSubs)

		cancel1s()
	}
}

func TestMaliciousAccessNode_NoHonestPeerScoring(t *testing.T) {
	total := 10
	success := 0
	for i := 0; i < total; i++ {
		if testGossipSubPartitionWhenNoHonestPeerScoring(t) {
			success++
		}
	}
	require.Less(t, success, total)
}

func testGossipSubPartitionWhenNoHonestPeerScoring(t *testing.T) bool {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)

	// two (honest) consensus nodes but with NO peer scoring enabled!
	con1Node, con1Id := p2pfixtures.NodeFixture(t, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus))
	con2Node, con2Id := p2pfixtures.NodeFixture(t, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus))

	// create > 2 * 12 malicious access nodes
	// 12 is the maximum size of default GossipSub mesh.
	// We want to make sure that it is unlikely for honest nodes to be in the same mesh (hence messages from
	// one honest node to the other is routed through the malicious nodes).
	accessNodeGroup, accessNodeIds := p2pfixtures.NodesFixture(t, sporkId, t.Name(), 30,
		p2pfixtures.WithRole(flow.RoleAccess),
		p2pfixtures.WithPeerScoringEnabled(idProvider),
		p2pfixtures.WithAppSpecificScore(maliciousAppSpecificScore(flow.IdentityList{&con1Id, &con2Id})))

	allNodes := append([]*p2pnode.Node{con1Node, con2Node}, accessNodeGroup...)
	allIds := append([]*flow.Identity{&con1Id, &con2Id}, accessNodeIds...)

	p2pfixtures.StartNodes(t, signalerCtx, allNodes, 100*time.Millisecond)
	defer p2pfixtures.StopNodes(t, allNodes, cancel, 100*time.Millisecond)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())

	// all nodes subscribe to block topic (common topic among all roles)
	_, err := con1Node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	con2Sub, err := con2Node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// access node group
	accessNodeSubs := make([]*pubsub.Subscription, len(accessNodeGroup))
	for i, node := range accessNodeGroup {
		accessNodeSubs[i], err = node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
		require.NoError(t, err)
	}

	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, allNodes, allIds)

	proposalMsg := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
	require.NoError(t, con1Node.Publish(ctx, blockTopic, proposalMsg))

	// we check that within a one-second window the message is not received by the other honest consensus node.
	// the one-second window is important because it triggers the heartbeat of the con1Node to perform a lazy pull (iHave).
	// And con1Node may randomly choose con2Node as the peer to perform the lazy pull.
	// However, when con2Node is not in the mesh of con1Node, it is deprived of the eager push from con1Node.
	// Hence, con2Node will not receive the message within a one-second window.
	ctx1s, cancel1s := context.WithTimeout(ctx, 1*time.Second)
	defer cancel1s()
	return p2pfixtures.HasSubReceivedMessage(t, ctx1s, proposalMsg, con2Sub)
}

func TestMaliciousAccessNodes_HonestPeerScoring(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)

	// two (honest) consensus nodes but WITH peer scoring enabled!
	con1Node, con1Id := p2pfixtures.NodeFixture(t, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerScoringEnabled(idProvider))
	con2Node, con2Id := p2pfixtures.NodeFixture(t, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerScoringEnabled(idProvider))

	// create > 2 * 12 malicious access nodes
	// 12 is the maximum size of default GossipSub mesh.
	// We want to make sure that it is unlikely for honest nodes to be in the same mesh (hence messages from
	// one honest node to the other is routed through the malicious nodes).
	accessNodeGroup, accessNodeIds := p2pfixtures.NodesFixture(t, sporkId, t.Name(), 30,
		p2pfixtures.WithRole(flow.RoleAccess),
		p2pfixtures.WithPeerScoringEnabled(idProvider),
		p2pfixtures.WithAppSpecificScore(maliciousAppSpecificScore(flow.IdentityList{&con1Id, &con2Id})))

	allNodes := append([]*p2pnode.Node{con1Node, con2Node}, accessNodeGroup...)
	allIds := append([]*flow.Identity{&con1Id, &con2Id}, accessNodeIds...)

	provider := id.NewFixedIdentityProvider(allIds)
	idProvider.On("ByPeerID", mocktestify.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		})

	p2pfixtures.StartNodes(t, signalerCtx, allNodes, 100*time.Millisecond)
	defer p2pfixtures.StopNodes(t, allNodes, cancel, 100*time.Millisecond)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())

	// all nodes subscribe to block topic (common topic among all roles)
	_, err := con1Node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	con2Sub, err := con2Node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// access node group
	accessNodeSubs := make([]*pubsub.Subscription, len(accessNodeGroup))
	for i, node := range accessNodeGroup {
		accessNodeSubs[i], err = node.Subscribe(blockTopic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
		require.NoError(t, err)
	}

	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, allNodes, allIds)

	proposalMsg := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
	require.NoError(t, con1Node.Publish(ctx, blockTopic, proposalMsg))

	// we check that within a one-second window the message is not received by the other honest consensus node.
	// the one-second window is important because it triggers the heartbeat of the con1Node to perform a lazy pull (iHave).
	// And con1Node may randomly choose con2Node as the peer to perform the lazy pull.
	// However, when con2Node is not in the mesh of con1Node, it is deprived of the eager push from con1Node.
	// Hence, con2Node will not receive the message within a one-second window.
	ctx1s, cancel1s := context.WithTimeout(ctx, 1*time.Second)
	defer cancel1s()
	p2pfixtures.SubMustReceiveMessage(t, ctx1s, proposalMsg, con2Sub)
}

// maliciousAppSpecificScore returns a malicious app specific score function that rewards the malicious node and
// punishes the honest nodes.
func maliciousAppSpecificScore(honestIds flow.IdentityList) func(peer.ID) float64 {
	honestIdProvider := id.NewFixedIdentityProvider(honestIds)
	return func(p peer.ID) float64 {
		_, isHonest := honestIdProvider.ByPeerID(p)
		if isHonest {
			return scoring.MaxAppSpecificPenalty
		}

		return scoring.MaxAppSpecificReward
	}
}
