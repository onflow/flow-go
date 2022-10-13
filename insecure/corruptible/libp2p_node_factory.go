package corruptible

import (
	"github.com/onflow/flow-go/network/p2p"
	"time"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
)

// CorruptibleLibP2PNodeFactory wrapper around the original DefaultLibP2PNodeFactory. Nodes returned from this factory func will be corrupted libp2p nodes.
func CorruptibleLibP2PNodeFactory(
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
	connectionPruning bool,
	updateInterval time.Duration,
) p2pbuilder.LibP2PFactoryFunc {
	return func() (p2p.LibP2PNode, error) {
		if chainID != flow.BftTestnet {
			panic("illegal chain id for using corruptible conduit factory")
		}

		builder := p2pbuilder.DefaultNodeBuilder(log, address, flowKey, sporkId, idProvider, metrics, resolver, role, peerScoringEnabled, connectionPruning, updateInterval)
		builder.SetCreateNode(NewCorruptibleLibP2PNode)
		return builder.Build()
	}
}
