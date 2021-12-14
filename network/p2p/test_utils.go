package p2p

import (
	"context"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
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
	builder := NewNodeBuilder(zerolog.Nop(), "0.0.0.0:0", networkKey, sporkID).
		SetPubSub(pubsub.NewGossipSub)

	for _, opt := range opts {
		opt(builder)
	}

	libp2pNode, err := builder.Build(context.TODO())
	require.NoError(t, err)

	return libp2pNode
}
