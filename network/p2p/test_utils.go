package p2p

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/unicast"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
)

type nodeOpt func(NodeBuilder)

func withSubscriptionFilter(filter pubsub.SubscriptionFilter) nodeOpt {
	return func(builder NodeBuilder) {
		builder.SetSubscriptionFilter(filter)
	}
}

func createNode(
	t *testing.T,
	nodeID flow.Identifier,
	networkKey crypto.PrivateKey,
	sporkID flow.Identifier,
	opts ...nodeOpt,
) *Node {
	builder := NewNodeBuilder(
		zerolog.Nop(),
		"0.0.0.0:0",
		networkKey,
		sporkID,
		validator.TopicValidatorFactory(nil),
	).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return NewDHT(c, h, unicast.FlowDHTProtocolID(sporkID))
		}).
		SetPubSub(pubsub.NewGossipSub)

	for _, opt := range opts {
		opt(builder)
	}

	libp2pNode, err := builder.Build(context.TODO())
	require.NoError(t, err)

	return libp2pNode
}
