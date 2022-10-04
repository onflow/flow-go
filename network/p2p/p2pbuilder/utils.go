package p2pbuilder

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network/p2p"
)

// notEjectedPeerFilter returns a PeerFilter that will return an error if the peer is unknown or ejected.
func notEjectedPeerFilter(idProvider id.IdentityProvider) p2p.PeerFilter {
	return func(p peer.ID) error {
		if id, found := idProvider.ByPeerID(p); !found {
			return fmt.Errorf("failed to get identity of unknown peer with peer id %s", p.String())
		} else if id.Ejected {
			return fmt.Errorf("peer %s with node id %s is ejected", p.String(), id.NodeID.String())
		}

		return nil
	}
}
