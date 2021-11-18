package p2p

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFilterSubscribe tests that if node X is filtered out on a specific channel by node Y's subscription
// filter, then node Y will never propagate any of node X's messages on that channel
func TestFilterSubscribe(t *testing.T) {
	// skip for now due to bug in libp2p gossipsub implementation:
	// https://github.com/libp2p/go-libp2p-pubsub/issues/449
	t.Skip()

	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	ids := flow.IdentityList{identity1, identity2}

	node1 := createNode(t, identity1.NodeID, privateKey1, sporkId, mockSubscriptionFilterPubsubOption(ids))
	node2 := createNode(t, identity2.NodeID, privateKey2, sporkId, mockSubscriptionFilterPubsubOption(ids))

	unstakedKey := unittest.NetworkingPrivKeyFixture()
	unstakedNode := createNode(t, flow.ZeroID, unstakedKey, sporkId)

	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(node2.Host())))
	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(unstakedNode.Host())))

	badTopic := engine.TopicFromChannel(engine.SyncCommittee, sporkId)

	sub1, err := node1.Subscribe(badTopic)
	require.NoError(t, err)

	sub2, err := node2.Subscribe(badTopic)
	require.NoError(t, err)

	unstakedSub, err := unstakedNode.Subscribe(badTopic)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(node1.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(node2.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.pubSub.ListPeers(badTopic.String())) > 0
	}, 1*time.Second, 100*time.Millisecond)

	// check that node1 and node2 don't accept unstakedNode as a peer
	require.Never(t, func() bool {
		for _, pid := range node1.pubSub.ListPeers(badTopic.String()) {
			if pid == unstakedNode.Host().ID() {
				return true
			}
		}
		return false
	}, 1*time.Second, 100*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	testPublish := func(wg *sync.WaitGroup, from *Node, sub *pubsub.Subscription) {
		data := []byte("hello")

		err := from.Publish(context.TODO(), badTopic, data)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		msg, err := sub.Next(ctx)
		cancel()
		require.NoError(t, err)
		require.Equal(t, msg.Data, data)

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

	wg.Wait()
}

// TestCanSubscribe tests that the subscription filter blocks a node from subscribing
// to channel that its role shouldn't subscribe to
func TestCanSubscribe(t *testing.T) {
	identity, privateKey := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	sporkId := unittest.IdentifierFixture()

	collectionNode := createNode(t, identity.NodeID, privateKey, sporkId, mockSubscriptionFilterPubsubOption(flow.IdentityList{identity}))
	defer func() {
		done, err := collectionNode.Stop()
		require.NoError(t, err)
		<-done
	}()

	goodTopic := engine.TopicFromChannel(engine.ProvideCollections, sporkId)
	_, err := collectionNode.pubSub.Join(goodTopic.String())
	require.NoError(t, err)

	var badTopic network.Topic
	allowedChannels := make(map[network.Channel]struct{})
	for _, ch := range engine.ChannelsByRole(flow.RoleCollection) {
		allowedChannels[ch] = struct{}{}
	}
	for _, ch := range engine.Channels() {
		if _, ok := allowedChannels[ch]; !ok {
			badTopic = engine.TopicFromChannel(ch, sporkId)
			break
		}
	}
	_, err = collectionNode.pubSub.Join(badTopic.String())
	require.Error(t, err)

	clusterTopic := engine.TopicFromChannel(engine.ChannelSyncCluster(flow.Emulator), sporkId)
	_, err = collectionNode.pubSub.Join(clusterTopic.String())
	require.NoError(t, err)
}

func mockSubscriptionFilterPubsubOption(ids flow.IdentityList) PubsubOption {
	idProvider := id.NewFixedIdentityProvider(ids)
	return func(_ context.Context, h host.Host) (pubsub.Option, error) {
		return pubsub.WithSubscriptionFilter(NewRoleBasedFilter(h.ID(), idProvider)), nil
	}
}
