package corruptnet

import (
	"time"

	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/network/p2p"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
)

// NewCorruptLibP2PNodeFactory wrapper around the original DefaultLibP2PNodeFactory. Nodes returned from this factory func will be corrupted libp2p nodes.
func NewCorruptLibP2PNodeFactory(
	log zerolog.Logger,
	chainID flow.ChainID,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metrics module.NetworkMetrics,
	resolver madns.BasicResolver,
	peerScoringEnabled bool,
	role string,
	onInterceptPeerDialFilters,
	onInterceptSecuredFilters []p2p.PeerFilter,
	connectionPruning bool,
	updateInterval time.Duration,
) p2pbuilder.LibP2PFactoryFunc {
	return func() (p2p.LibP2PNode, error) {
		if chainID != flow.BftTestnet {
			panic("illegal chain id for using corruptible conduit factory")
		}

		builder := p2pbuilder.DefaultNodeBuilder(
			log,
			address,
			flowKey,
			sporkId,
			idProvider,
			metrics,
			resolver,
			role,
			onInterceptPeerDialFilters,
			onInterceptSecuredFilters,
			peerScoringEnabled,
			connectionPruning,
			updateInterval)
		builder.SetCreateNode(NewCorruptLibP2PNode)
		internal.OverrideWithCorruptGossipSub(builder)
		return builder.Build()
	}
}
