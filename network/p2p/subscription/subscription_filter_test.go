package subscription_test

import (
	"context"
	"sync"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/subscription"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFilterSubscribe tests that if node X is filtered out on a specific channel by node Y's subscription
// filter, then node Y will never propagate any of node X's messages on that channel
func TestFilterSubscribe(t *testing.T) {
	// TODO: skip for now due to bug in libp2p gossipsub implementation:
	// https://github.com/libp2p/go-libp2p-pubsub/issues/449
	unittest.SkipUnless(t, unittest.TEST_TODO, "skip for now due to bug in libp2p gossipsub implementation: https://github.com/libp2p/go-libp2p-pubsub/issues/449")

	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	ids := flow.IdentityList{identity1, identity2}

	node1 := p2pfixtures.CreateNode(t, privateKey1, sporkId, zerolog.Nop(), ids, p2pfixtures.WithSubscriptionFilter(subscriptionFilter(identity1, ids)))
	node2 := p2pfixtures.CreateNode(t, privateKey2, sporkId, zerolog.Nop(), ids, p2pfixtures.WithSubscriptionFilter(subscriptionFilter(identity2, ids)))

	unstakedKey := unittest.NetworkingPrivKeyFixture()
	unstakedNode := p2pfixtures.CreateNode(t, unstakedKey, sporkId, zerolog.Nop(), ids)

	require.NoError(t, node1.ConnectToPeer(context.TODO(), *host.InfoFromHost(node2.Host())))
	require.NoError(t, node1.ConnectToPeer(context.TODO(), *host.InfoFromHost(unstakedNode.Host())))

	badTopic := channels.TopicFromChannel(channels.SyncCommittee, sporkId)

	logger := unittest.Logger()
	topicValidator := flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter())

	sub1, err := node1.Subscribe(badTopic, topicValidator)
	require.NoError(t, err)

	sub2, err := node2.Subscribe(badTopic, topicValidator)
	require.NoError(t, err)

	unstakedSub, err := unstakedNode.Subscribe(badTopic, topicValidator)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(node1.ListPeers(badTopic.String())) > 0 &&
			len(node2.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.ListPeers(badTopic.String())) > 0
	}, 1*time.Second, 100*time.Millisecond)

	// check that node1 and node2 don't accept unstakedNode as a peer
	require.Never(t, func() bool {
		for _, pid := range node1.ListPeers(badTopic.String()) {
			if pid == unstakedNode.ID() {
				return true
			}
		}
		return false
	}, 1*time.Second, 100*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	testPublish := func(wg *sync.WaitGroup, from p2p.LibP2PNode, sub p2p.Subscription) {

		outgoingMessageScope, err := message.NewOutgoingScope(
			ids.NodeIDs(),
			channels.TopicFromChannel(channels.SyncCommittee, sporkId),
			[]byte("hello"),
			unittest.NetworkCodec().Encode,
			message.ProtocolTypePubSub)
		require.NoError(t, err)

		err = from.Publish(context.TODO(), outgoingMessageScope)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		msg, err := sub.Next(ctx)
		cancel()
		require.NoError(t, err)

		expectedReceivedData, err := outgoingMessageScope.Proto().Marshal()
		require.NoError(t, err)
		require.Equal(t, msg.Data, expectedReceivedData)

		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		_, err = unstakedSub.Next(ctx)
		cancel()
		require.ErrorIs(t, err, context.DeadlineExceeded)

		wg.Done()
	}

	// publish a message from node 1 and check that only node2 receives
	testPublish(&wg, node1, sub2)

	// publish a message from node 2 and check that only node1 receives
	testPublish(&wg, node2, sub1)

	unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "timeout performing publish test")
}

// TestCanSubscribe tests that the subscription filter blocks a node from subscribing
// to channel that its role shouldn't subscribe to
func TestCanSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	identity, privateKey := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	sporkId := unittest.IdentifierFixture()

	collectionNode := p2pfixtures.CreateNode(t,
		privateKey,
		sporkId,
		zerolog.Nop(),
		flow.IdentityList{identity},
		p2pfixtures.WithSubscriptionFilter(subscriptionFilter(identity, flow.IdentityList{identity})))

	p2ptest.StartNode(t, signalerCtx, collectionNode)
	defer p2ptest.StopNode(t, collectionNode, cancel)

	logger := unittest.Logger()
	topicValidator := flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter())

	goodTopic := channels.TopicFromChannel(channels.ProvideCollections, sporkId)
	_, err := collectionNode.Subscribe(goodTopic, topicValidator)
	require.NoError(t, err)

	var badTopic channels.Topic
	allowedChannels := make(map[channels.Channel]struct{})
	for _, ch := range channels.ChannelsByRole(flow.RoleCollection) {
		allowedChannels[ch] = struct{}{}
	}
	for _, ch := range channels.Channels() {
		if _, ok := allowedChannels[ch]; !ok {
			badTopic = channels.TopicFromChannel(ch, sporkId)
			break
		}
	}
	_, err = collectionNode.Subscribe(badTopic, topicValidator)
	require.Error(t, err)

	clusterTopic := channels.TopicFromChannel(channels.SyncCluster(flow.Emulator), sporkId)
	_, err = collectionNode.Subscribe(clusterTopic, topicValidator)
	require.NoError(t, err)
}

func subscriptionFilter(self *flow.Identity, ids flow.IdentityList) pubsub.SubscriptionFilter {
	idProvider := id.NewFixedIdentityProvider(ids)
	return subscription.NewRoleBasedFilter(self.Role, idProvider)
}
