package p2p_test

import (
	"context"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/test"
)

const maxConnectAttempt = 3

type nodeOpt func(p2p.NodeBuilder)

func withSubscriptionFilter(filter pubsub.SubscriptionFilter) nodeOpt {
	return func(builder p2p.NodeBuilder) {
		builder.SetSubscriptionFilter(filter)
	}
}

func createNode(t *testing.T, nodeID flow.Identifier, networkKey crypto.PrivateKey, sporkID flow.Identifier, logger zerolog.Logger, opts ...nodeOpt) *p2p.Node {
	builder := p2p.NewNodeBuilder(logger, "0.0.0.0:0", networkKey, sporkID).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return dht.NewDHT(c, h, unicast.FlowDHTProtocolID(sporkID), zerolog.Nop(), metrics.NewNoopCollector())
		}).
		SetPubSub(pubsub.NewGossipSub).
		SetResourceManager(test.NewResourceManager(t))

	for _, opt := range opts {
		opt(builder)
	}

	libp2pNode, err := builder.Build(context.TODO())
	require.NoError(t, err)

	return libp2pNode
}
