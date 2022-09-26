package corruptible

import (
	"context"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// CorruptedLibP2PNodeFactory wrapper around the original DefaultLibP2PNodeFactory. Nodes returned from this factory func will be corrupted libp2p nodes.
func CorruptedLibP2PNodeFactory(
	log zerolog.Logger,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider id.IdentityProvider,
	metrics module.NetworkMetrics,
	resolver madns.BasicResolver,
	role string,
) p2pbuilder.LibP2PFactoryFunc {
	return func(ctx context.Context) (p2pnode.LibP2PNode, error) {
		// get the default node builder and corrupt it by overriding the create node func
		builder := p2pbuilder.DefaultNodeBuilder(log, address, flowKey, sporkId, idProvider, metrics, resolver, role)
		builder.SetCreateNode(NewCorruptibleNode)
		return builder.Build(ctx)
	}
}
