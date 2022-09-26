package corruptible

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p/unicast"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// LibP2PNodeFactory wrapper around the original DefaultLibP2PNodeFactory. Nodes returned from this factory func will be corrupted libp2p nodes.
func LibP2PNodeFactory(
	log zerolog.Logger,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider id.IdentityProvider,
	metrics module.NetworkMetrics,
	resolver madns.BasicResolver,
	role string,
	topicValidator pubsub.ValidatorEx,
) p2pbuilder.LibP2PFactoryFunc {
	return func(ctx context.Context) (p2pnode.LibP2PNode, error) {

		// get the default node builder and corrupt it by overriding the create node func
		builder := p2pbuilder.DefaultNodeBuilder(log, address, flowKey, sporkId, idProvider, metrics, resolver, role)
		builder.SetCreateNode(func(logger zerolog.Logger, host host.Host, pubSub *pubsub.PubSub, routing routing.Routing, pCache *p2pnode.ProtocolPeerCache, uniMgr *unicast.Manager) p2pnode.LibP2PNode {
			node := p2pnode.NewNode(logger, host, pubSub, routing, pCache, uniMgr)
			return &P2PNode{
				Node:           node,
				topicValidator: topicValidator,
			}
		})
		return builder.Build(ctx)
	}
}
