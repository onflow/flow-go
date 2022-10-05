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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccessNodeScore_Integration(t *testing.T) {
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
	p2pfixtures.SubsMustNeverReceiveAnyMessage(t, ctx1s, groupTwoSubs)

	//// now consensus node is doing something very bad!
	//// it is subscribing to a channel that it is not supposed to subscribe to.
	//conSubChunks, err := conNode.Subscribe(channels.TopicFromChannel(channels.RequestChunks, sporkId), unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	//require.NoError(t, err)
	//
	//// let's wait for a bit to subscription propagate.
	//time.Sleep(5 * time.Second)
	//
	//// consensus node publishes another proposal, but this time, it should not reach verification node.
	//// since upon an unauthorized subscription, verification node should have slashed consensus node on
	//// the GossipSub scoring protocol.
	//proposalMsg = p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture())
	//// publishes a message to the topic.
	//require.NoError(t, conNode.Publish(ctx, blockTopic, proposalMsg))
	//
	//ctx5s, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	//defer cancel5s()
	//p2pfixtures.SubsMustNeverReceiveAnyMessage(t, ctx5s, []*pubsub.Subscription{ver1SubBlocks, ver2SubBlocks})
	//
	//// moreover, a verification node publishing a message to the request chunk topic should not reach consensus node.
	//// however, both verification nodes should receive the message.
	//chunkDataPackRequestMsg := p2pfixtures.MustEncodeEvent(t, &messages.ChunkDataRequest{
	//	ChunkID: unittest.IdentifierFixture(),
	//	Nonce:   rand.Uint64(),
	//})
	//require.NoError(t, verNode1.Publish(ctx, channels.TopicFromChannel(channels.RequestChunks, sporkId), chunkDataPackRequestMsg))
	//
	//ctx1s, cancel1s = context.WithTimeout(ctx, 1*time.Second)
	//defer cancel1s()
	//p2pfixtures.SubsMustReceiveMessage(t, ctx1s, chunkDataPackRequestMsg, []*pubsub.Subscription{ver1SubChunks, ver2SubChunks})
	//
	//ctx5s, cancel5s = context.WithTimeout(ctx, 5*time.Second)
	//defer cancel5s()
	//p2pfixtures.SubsMustNeverReceiveAnyMessage(t, ctx5s, []*pubsub.Subscription{conSubChunks})
}
