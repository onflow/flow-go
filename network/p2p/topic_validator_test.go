package p2p

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/message"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTopicValidator tests that a topic validator prevents an unstaked node to send messages to any staked node
func TestTopicValidator(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	// create two staked nodes - node1 and node2
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	node1 := createNode(t, identity1.NodeID, privateKey1, sporkId)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	node2 := createNode(t, identity2.NodeID, privateKey2, sporkId)

	badTopic := engine.TopicFromChannel(engine.SyncCommittee, sporkId)

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

	unstakedKey := unittest.NetworkingPrivKeyFixture()
	require.NoError(t, err)
	// create one unstaked node
	unstakedNode := createNode(t, flow.ZeroID, unstakedKey, sporkId)
	require.NoError(t, err)

	// node1 is connected to node2, and the unstaked node is connected to node1
	// unstaked Node <-> node1 <-> node2
	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(node2.host)))
	require.NoError(t, unstakedNode.AddPeer(context.TODO(), *host.InfoFromHost(node1.host)))

	// node1 and node2 subscribe to the topic with the topic validator
	sub1, err := node1.Subscribe(badTopic, stakedValidator)
	require.NoError(t, err)
	sub2, err := node2.Subscribe(badTopic, stakedValidator)
	require.NoError(t, err)
	// the unstaked node subscribes to the topic WITHOUT the topic validator
	unstakedSub, err := unstakedNode.Subscribe(badTopic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(node1.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(node2.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.pubSub.ListPeers(badTopic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()

	m1 := message.Message{
		Payload: []byte("hello1"),
	}
	data1, err := m1.Marshal()
	require.NoError(t, err)

	// node2 publishes a message
	err = node2.Publish(timedCtx, badTopic, data1)
	require.NoError(t, err)

	// node1 gets the message
	msg, err := sub1.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data1)

	// node2 also gets the message (as part of the libp2p loopback of published topic messages)
	msg, err = sub2.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data1)

	// the unstaked node also gets the message since it subscribed to the channel without the topic validator
	msg, err = unstakedSub.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data1)

	timedCtx, cancel2s := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2s()

	m2 := message.Message{
		Payload: []byte("hello2"),
	}
	data2, err := m2.Marshal()
	require.NoError(t, err)

	// the unstaked node now publishes a message
	err = unstakedNode.Publish(timedCtx, badTopic, data2)
	require.NoError(t, err)

	// it receives its own message
	msg, err = unstakedSub.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data2)

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

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")
}
