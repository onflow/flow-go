package p2p

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/message"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTopicValidator tests that a topic validator prevents an unstaked node to send messages to any staked node
func TestTopicValidator(t *testing.T) {

	// create two staked nodes - node1 and node2
	identity1, privateKey1 := createID(t)
	node1 := createNode(t, identity1.NodeID, privateKey1)

	identity2, privateKey2 := createID(t)
	node2 := createNode(t, identity2.NodeID, privateKey2)

	badTopic := engine.TopicFromChannel(engine.SyncCommittee, rootBlockID)

	ids := flow.IdentityList{identity1, identity2}
	translator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	stakedValidator := validator.StakedValidator(func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}
		return ids.ByNodeID(fid)
	})

	unstakedKey, err := unittest.NetworkingKey()
	require.NoError(t, err)
	// create one unstaked node
	unstakedNode := createNode(t, flow.ZeroID, unstakedKey)
	require.NoError(t, err)

	// node1 is connected to node2, and the unstaked node is connected to node1
	// unstaked Node <-> node1 <-> node2
	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(node2.host)))
	require.NoError(t, unstakedNode.AddPeer(context.TODO(), *host.InfoFromHost(node1.host)))

	// node1 and node2 subscribe to the topic with the topic validator
	sub1, err := node1.Subscribe(context.TODO(), badTopic, stakedValidator)
	require.NoError(t, err)
	sub2, err := node2.Subscribe(context.TODO(), badTopic, stakedValidator)
	require.NoError(t, err)
	// the unstaked node subscribes to the topic WITHOUT the topic validator
	unstakedSub, err := unstakedNode.Subscribe(context.TODO(), badTopic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(node1.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(node2.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.pubSub.ListPeers(badTopic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	m := message.Message{
		Payload: []byte("hello"),
	}
	data, err := m.Marshal()
	require.NoError(t, err)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()

	// node2 publishes a message
	err = node2.Publish(timedCtx, badTopic, data)
	require.NoError(t, err)

	// node1 gets the message
	msg, err := sub1.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	// node2 also gets the message (as part of the libp2p loopback of published topic messages)
	msg, err = sub2.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	// the unstaked node also gets the message since it subscribed to the channel without the topic validator
	msg, err = unstakedSub.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	timedCtx, cancel2s := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2s()

	// the unstaked node now publishes a message
	err = unstakedNode.Publish(timedCtx, badTopic, data)
	require.NoError(t, err)

	// it receives its own message
	msg, err = unstakedSub.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	// node 1 does NOT receive the message due to the topic validator
	var wg sync.WaitGroup
	wg.Add(1)
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		msg, err = sub1.Next(timedCtx)
		require.Error(t, err)
		wg.Done()
	}()

	// node 2 also does not receive the message via gossip from the node1 (event after the 1 second hearbeat)
	wg.Add(1)
	timedCtx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		msg, err = sub2.Next(timedCtx)
		require.Error(t, err)
		wg.Done()
	}()

	wg.Wait()
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
) *Node {
	libp2pNode, err := NewDefaultLibP2PNodeBuilder(nodeID, "0.0.0.0:0", networkKey).
		SetRootBlockID(rootBlockID).
		SetPubsubOptions(DefaultPubsubOptions(DefaultMaxPubSubMsgSize)...).
		Build(context.TODO())
	require.NoError(t, err)

	return libp2pNode
}
