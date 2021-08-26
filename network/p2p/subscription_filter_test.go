package p2p

import (
	"context"
	"fmt"
	"testing"

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

func TestCanSubscribe(t *testing.T) {
	identity, privateKey := createID(t)
	rootBlockID := unittest.IdentifierFixture()

	node := createNode(t, rootBlockID, identity.NodeID, privateKey, createSubscriptionFilterPubsubOption(t, flow.IdentityList{identity}, rootBlockID))

	badTopic := getDisallowedTopic(t, identity, rootBlockID)
	_, err := node.pubSub.Join(badTopic.String())

	fmt.Println(err)
	require.Error(t, err)
}

func getDisallowedTopic(t *testing.T, id *flow.Identity, rootBlockID flow.Identifier) network.Topic {
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

func createSubscriptionFilterPubsubOption(t *testing.T, ids flow.IdentityList, rootBlockID flow.Identifier) PubsubOption {
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
	rootBlockID flow.Identifier,
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
