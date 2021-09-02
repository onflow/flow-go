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
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTopicValidator(t *testing.T) {
	identity1, privateKey1 := createID(t)
	node1 := createNode(t, identity1.NodeID, privateKey1)

	identity2, privateKey2 := createID(t)
	node2 := createNode(t, identity2.NodeID, privateKey2)

	badTopic := engine.TopicFromChannel(engine.SyncCommittee, rootBlockID)

	ids := flow.IdentityList{identity1, identity2}
	translator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	validator := StakedValidator{func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}
		return ids.ByNodeID(fid)
	}}

	unstakedKey, err := unittest.NetworkingKey()
	require.NoError(t, err)
	unstakedNode := createNode(t, flow.ZeroID, unstakedKey)
	require.NoError(t, err)

	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(node2.host)))
	require.NoError(t, unstakedNode.AddPeer(context.TODO(), *host.InfoFromHost(node1.host)))

	sub1, err := node1.Subscribe(context.TODO(), badTopic, validator.Validate)
	require.NoError(t, err)
	sub2, err := node2.Subscribe(context.TODO(), badTopic, validator.Validate)
	require.NoError(t, err)
	unstakedSub, err := unstakedNode.Subscribe(context.TODO(), badTopic)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(node1.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(node2.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.pubSub.ListPeers(badTopic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	data := []byte("hello")

	timedCtx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	err = node2.Publish(timedCtx, badTopic, data)
	require.NoError(t, err)

	msg, err := sub1.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	msg, err = sub2.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	msg, err = unstakedSub.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	timedCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = unstakedNode.Publish(timedCtx, badTopic, data)
	require.NoError(t, err)

	msg, err = unstakedSub.Next(timedCtx)
	require.NoError(t, err)
	require.Equal(t, msg.Data, data)

	var wg sync.WaitGroup
	wg.Add(2)
	timedCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		msg, err = sub1.Next(timedCtx)
		require.Error(t, err)
		wg.Done()
	}()

	timedCtx, cancel = context.WithTimeout(context.Background(), time.Second)
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
