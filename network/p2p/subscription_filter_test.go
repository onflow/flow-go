package p2p

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestFilterSubscribe(t *testing.T) {
	identity1, privateKey1 := createID(t)
	identity2, privateKey2 := createID(t)
	ids := flow.IdentityList{identity1, identity2}

	node1 := createNode(t, identity1.NodeID, privateKey1, createSubscriptionFilterPubsubOption(t, ids))
	node2 := createNode(t, identity2.NodeID, privateKey2, createSubscriptionFilterPubsubOption(t, ids))

	unstakedKey, err := unittest.NetworkingKey()
	require.NoError(t, err)
	unstakedNode := createNode(t, flow.ZeroID, unstakedKey)

	fmt.Println(node1.host.ID())
	fmt.Println(node2.host.ID())
	fmt.Println(unstakedNode.host.ID())

	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(node2.Host())))
	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(unstakedNode.Host())))

	badTopic := engine.TopicFromChannel(engine.SyncCommittee, rootBlockID)

	sub1, err := node1.Subscribe(context.TODO(), badTopic)
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)
	sub2, err := node2.Subscribe(context.TODO(), badTopic)
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)
	unstakedSub, err := unstakedNode.Subscribe(context.TODO(), badTopic)
	require.NoError(t, err)
	time.Sleep(300 * time.Millisecond)

	require.Eventually(t, func() bool {
		return len(node1.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(node2.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.pubSub.ListPeers(badTopic.String())) > 0
	}, 1*time.Second, 100*time.Millisecond)

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

		from.Publish(context.TODO(), badTopic, data)

		fmt.Println(from.pubSub.ListPeers(badTopic.String()))

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		msg, err := sub.Next(ctx)
		cancel()
		require.NoError(t, err)
		require.Equal(t, msg.Data, data)

		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		msg, err = unstakedSub.Next(ctx)
		fmt.Println(msg)
		cancel()
		require.ErrorIs(t, err, context.DeadlineExceeded)

		wg.Done()
	}

	// publish a message from node 1 and check that only node2 receives
	testPublish(&wg, node1, sub2)

	// publish a message from node 2 and check that only node1 receives
	testPublish(&wg, node2, sub1)

	fmt.Println(sub2)

	wg.Wait()
}

func TestCanSubscribe(t *testing.T) {
	identity, privateKey := createID(t)

	node := createNode(t, identity.NodeID, privateKey, createSubscriptionFilterPubsubOption(t, flow.IdentityList{identity}))

	badTopic := getDisallowedTopic(t, identity)
	_, err := node.pubSub.Join(badTopic.String())

	require.Error(t, err)
}

func getDisallowedTopic(t *testing.T, id *flow.Identity) network.Topic {
	allowedChannels := engine.UnstakedChannels()
	if id != nil {
		allowedChannels = engine.ChannelsByRole(id.Role)
	}

	for _, ch := range engine.Channels() {
		if !allowedChannels.Contains(ch) {
			return engine.TopicFromChannel(ch, rootBlockID)
		}
	}

	require.FailNow(t, "could not find disallowed topic for role %s", id.Role)

	return ""
}

func createSubscriptionFilterPubsubOption(t *testing.T, ids flow.IdentityList) PubsubOption {
	idTranslator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	idProvider := id.NewFixedIdentityProvider(ids)

	return func(_ context.Context, h host.Host) (pubsub.Option, error) {
		return pubsub.WithSubscriptionFilter(NewSubscriptionFilter(h.ID(), rootBlockID, idProvider, idTranslator)), nil
	}
}

func createID(t *testing.T) (*flow.Identity, crypto.PrivateKey) {
	networkKey, err := unittest.NetworkingKey()
	require.NoError(t, err)
	id := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleAccess),
		unittest.WithNetworkingKey(networkKey.PublicKey()),
	)
	return id, networkKey
}

func createNode(
	t *testing.T,
	nodeID flow.Identifier,
	networkKey crypto.PrivateKey,
	psOpts ...PubsubOption,
) *Node {
	libp2pNode, err := NewDefaultLibP2PNodeBuilder(nodeID, "0.0.0.0:0", networkKey).
		SetRootBlockID(rootBlockID).
		SetPubsubOptions(psOpts...).
		Build(context.TODO())
	require.NoError(t, err)

	return libp2pNode
}
