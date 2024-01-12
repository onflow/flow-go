package scoring_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
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
	groupOneNodes, groupOneIds := p2ptest.NodesFixture(t, sporkId, t.Name(), 5,
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))
	groupTwoNodes, groupTwoIds := p2ptest.NodesFixture(t, sporkId, t.Name(), 5,
		idProvider,
		p2ptest.WithRole(flow.RoleCollection))
	accessNodeGroup, accessNodeIds := p2ptest.NodesFixture(t, sporkId, t.Name(), 5,
		idProvider,
		p2ptest.WithRole(flow.RoleAccess))

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
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	logger := unittest.Logger()

	// all nodes subscribe to block topic (common topic among all roles)
	// group one
	groupOneSubs := make([]p2p.Subscription, len(groupOneNodes))
	var err error
	for i, node := range groupOneNodes {
		groupOneSubs[i], err = node.Subscribe(blockTopic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
	}
	// group two
	groupTwoSubs := make([]p2p.Subscription, len(groupTwoNodes))
	for i, node := range groupTwoNodes {
		groupTwoSubs[i], err = node.Subscribe(blockTopic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
	}
	// access node group
	accessNodeSubs := make([]p2p.Subscription, len(accessNodeGroup))
	for i, node := range accessNodeGroup {
		accessNodeSubs[i], err = node.Subscribe(blockTopic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
	}

	// creates a topology as follows:
	// groupOneNodes <--> accessNodeGroup <--> groupTwoNodes
	p2ptest.LetNodesDiscoverEachOther(t, ctx, append(groupOneNodes, accessNodeGroup...), append(groupOneIds, accessNodeIds...))
	p2ptest.LetNodesDiscoverEachOther(t, ctx, append(groupTwoNodes, accessNodeGroup...), append(groupTwoIds, accessNodeIds...))

	// checks end-to-end message delivery works
	// each node sends a distinct message to all and checks that all nodes receive it.
	for _, node := range nodes {
		outgoingMessageScope, err := message.NewOutgoingScope(
			ids.NodeIDs(),
			channels.TopicFromChannel(channels.PushBlocks, sporkId),
			unittest.ProposalFixture(),
			unittest.NetworkCodec().Encode,
			message.ProtocolTypePubSub)
		require.NoError(t, err)
		require.NoError(t, node.Publish(ctx, outgoingMessageScope))

		// checks that the message is received by all nodes.
		ctx1s, cancel1s := context.WithTimeout(ctx, 5*time.Second)
		expectedReceivedData, err := outgoingMessageScope.Proto().Marshal()
		require.NoError(t, err)
		p2pfixtures.SubsMustReceiveMessage(t, ctx1s, expectedReceivedData, groupOneSubs)
		p2pfixtures.SubsMustReceiveMessage(t, ctx1s, expectedReceivedData, accessNodeSubs)
		p2pfixtures.SubsMustReceiveMessage(t, ctx1s, expectedReceivedData, groupTwoSubs)

		cancel1s()
	}
}

// TestFullGossipSubConnectivityAmongHonestNodesWithMaliciousMajority tests pushing access nodes to the edges of the network.
// This test proves that if access nodes are PUSHED to the edge of the network, even their malicious majority cannot partition
// the network of honest nodes.
// The scenario tests that whether two honest nodes are in each others topic mesh on GossipSub
// when the network topology is a complete graph (i.e., full topology) and a malicious majority of access nodes are present.
// The honest nodes (i.e., non-Access nodes) are enabled with peer scoring, then the honest nodes are enabled with peer scoring.
func TestFullGossipSubConnectivityAmongHonestNodesWithMaliciousMajority(t *testing.T) {
	// Note: if this test is ever flaky, this means a bug in our scoring system. Please escalate to the team instead of skipping.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()

	idProvider := mock.NewIdentityProvider(t)
	defaultConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	// override the default config to make the mesh tracer log more frequently
	defaultConfig.NetworkConfig.GossipSub.RpcTracer.LocalMeshLogInterval = time.Second

	con1Node, con1Id := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.OverrideFlowConfig(defaultConfig))
	con2Node, con2Id := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.OverrideFlowConfig(defaultConfig))

	// create > 2 * 12 malicious access nodes
	// 12 is the maximum size of default GossipSub mesh.
	// We want to make sure that it is unlikely for honest nodes to be in the same mesh without peer scoring.
	accessNodeGroup, accessNodeIds := p2ptest.NodesFixture(t, sporkId, t.Name(), 30,
		idProvider,
		p2ptest.WithRole(flow.RoleAccess),
		// overrides the default peer scoring parameters to mute GossipSub traffic from/to honest nodes.
		p2ptest.EnablePeerScoringWithOverride(&p2p.PeerScoringConfigOverride{
			AppSpecificScoreParams: maliciousAppSpecificScore(flow.IdentityList{&con1Id, &con2Id}, defaultConfig.NetworkConfig.GossipSub.ScoringParameters.InternalPeerScoring),
		}),
	)

	allNodes := append([]p2p.LibP2PNode{con1Node, con2Node}, accessNodeGroup...)
	allIds := append(flow.IdentityList{&con1Id, &con2Id}, accessNodeIds...)

	provider := id.NewFixedIdentityProvider(allIds)
	idProvider.On("ByPeerID", mocktestify.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		}).Maybe()

	p2ptest.StartNodes(t, signalerCtx, allNodes)
	defer p2ptest.StopNodes(t, allNodes, cancel)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	// all nodes subscribe to block topic (common topic among all roles)
	_, err = con1Node.Subscribe(blockTopic, flowpubsub.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	_, err = con2Node.Subscribe(blockTopic, flowpubsub.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// access node group
	accessNodeSubs := make([]p2p.Subscription, len(accessNodeGroup))
	for i, node := range accessNodeGroup {
		sub, err := node.Subscribe(blockTopic, flowpubsub.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
		require.NoError(t, err, "access node %d failed to subscribe to block topic", i)
		accessNodeSubs[i] = sub
	}

	// let nodes reside on a full topology, hence no partition is caused by the topology.
	p2ptest.LetNodesDiscoverEachOther(t, ctx, allNodes, allIds)

	// checks whether con1 and con2 are in the same mesh
	tick := time.Second        // Set the tick duration as needed
	timeout := 5 * time.Second // Set the timeout duration as needed

	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)

	con1HasCon2 := false // denotes whether con1 has con2 in its mesh
	con2HasCon1 := false // denotes whether con2 has con1 in its mesh
	for {
		select {
		case <-ticker.C:
			con1BlockTopicPeers := con1Node.GetLocalMeshPeers(blockTopic)
			for _, p := range con1BlockTopicPeers {
				if p == con2Node.ID() {
					con2HasCon1 = true
					break // con1 has con2 in its mesh, break out of the current loop
				}
			}

			con2BlockTopicPeers := con2Node.GetLocalMeshPeers(blockTopic)
			for _, p := range con2BlockTopicPeers {
				if p == con1Node.ID() {
					con1HasCon2 = true
					break // con2 has con1 in its mesh, break out of the current loop
				}
			}

			if con2HasCon1 && con1HasCon2 {
				return
			}

		case <-timeoutCh:
			require.Fail(t, "timed out waiting for con1 to have con2 in its mesh; honest nodes are not on each others' topic mesh on GossipSub")
		}
	}
}

// maliciousAppSpecificScore returns a malicious app specific penalty function that rewards the malicious node and
// punishes the honest nodes.
func maliciousAppSpecificScore(honestIds flow.IdentityList, optionCfg p2pconfig.InternalPeerScoring) func(peer.ID) float64 {
	honestIdProvider := id.NewFixedIdentityProvider(honestIds)
	return func(p peer.ID) float64 {
		_, isHonest := honestIdProvider.ByPeerID(p)
		if isHonest {
			return optionCfg.Penalties.MaxAppSpecificPenalty
		}

		return optionCfg.Rewards.MaxAppSpecificReward
	}
}
