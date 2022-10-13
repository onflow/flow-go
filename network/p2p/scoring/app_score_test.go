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
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccessNodeScore_Integration_HonestANs(t *testing.T) {
	ctx := context.Background()
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	// one consensus node.
	groupOneNodes, groupOneIds := p2pfixtures.NodesFixture(t, ctx, sporkId, t.Name(), 12,
		p2pfixtures.WithRole(flow.RoleConsensus),
		p2pfixtures.WithPeerScoringEnabled(idProvider))
	groupTwoNodes, groupTwoIds := p2pfixtures.NodesFixture(t, ctx, sporkId, t.Name(), 12,
		p2pfixtures.WithRole(flow.RoleConsensus),
		p2pfixtures.WithPeerScoringEnabled(idProvider))
	accessNodeGroup, accessNodeIds := p2pfixtures.NodesFixture(t, ctx, sporkId, t.Name(), 12,
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

	defer p2pfixtures.StopNodes(t, nodes)

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

	proposalMsg := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
	require.NoError(t, groupOneNodes[0].Publish(ctx, blockTopic, proposalMsg))

	time.Sleep(5 * time.Second)

	// checks that the message is received by all nodes.
	ctx1s, cancel1s := context.WithTimeout(ctx, 1*time.Second)
	defer cancel1s()
	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, groupOneSubs)
	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, accessNodeSubs)
	p2pfixtures.SubsMustReceiveMessage(t, ctx1s, proposalMsg, groupTwoSubs)
}

func TestMaliciousAccessNodes_NoHonestPeerScoring(t *testing.T) {
	ctx := context.Background()
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)

	// two (honest) consensus nodes but with NO peer scoring enabled!
	con1Node, con1Id := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerScoringEnabled(idProvider), p2pfixtures.WithAppSpecificScore(pushConsensusNodesToEdge(idProvider)))
	con2Node, con2Id := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerScoringEnabled(idProvider), p2pfixtures.WithAppSpecificScore(pushConsensusNodesToEdge(idProvider)))

	// create > 2 * 12 malicious access nodes
	// 12 is the maximum size of default GossipSub mesh.
	// We want to make sure that it is unlikely for honest nodes to be in the same mesh (hence messages from
	// one honest node to the other is routed through the malicious nodes).
	accessNodeGroup, accessNodeIds := p2pfixtures.NodesFixture(t, ctx, sporkId, t.Name(), 30,
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

	defer p2pfixtures.StopNodes(t, allNodes)

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
	p2pfixtures.SubMustNeverReceiveAnyMessage(t, ctx1s, con2Sub)
}

func TestMaliciousAccessNodes_HonestPeerScoring(t *testing.T) {
	ctx := context.Background()
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)

	// two (honest) consensus nodes but WITH peer scoring enabled!
	con1Node, con1Id := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerScoringEnabled(idProvider))
	con2Node, con2Id := p2pfixtures.NodeFixture(t, ctx, sporkId, t.Name(), p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerScoringEnabled(idProvider))

	// create > 2 * 12 malicious access nodes
	// 12 is the maximum size of default GossipSub mesh.
	// We want to make sure that it is unlikely for honest nodes to be in the same mesh (hence messages from
	// one honest node to the other is routed through the malicious nodes).
	accessNodeGroup, accessNodeIds := p2pfixtures.NodesFixture(t, ctx, sporkId, t.Name(), 30,
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

	defer p2pfixtures.StopNodes(t, allNodes)

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

func pushConsensusNodesToEdge(idProvider module.IdentityProvider) func(peer.ID) float64 {
	return func(p peer.ID) float64 {
		identity, _ := idProvider.ByPeerID(p)
		if identity.Role == flow.RoleConsensus {
			return scoring.MinAppSpecificReward
		}

		return scoring.MaxAppSpecificReward
	}
}
