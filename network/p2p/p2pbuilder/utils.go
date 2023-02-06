package p2pbuilder

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
)

// notEjectedPeerFilter returns a PeerFilter that will return an error if the peer is unknown or ejected.
func notEjectedPeerFilter(idProvider module.IdentityProvider) p2p.PeerFilter {
	return func(p peer.ID) error {
		if id, found := idProvider.ByPeerID(p); !found {
			return fmt.Errorf("failed to get identity of unknown peer with peer id %s", p.String())
		} else if id.Ejected {
			return fmt.Errorf("peer %s with node id %s is ejected", p.String(), id.NodeID.String())
		}

		return nil
	}
}

func appendBaseLimitLogger(prefix string, baseLimit rcmgr.BaseLimit, logger zerolog.Logger) zerolog.Logger {
	return logger.With().
		Int(fmt.Sprintf("%s_streams", prefix), baseLimit.Streams).
		Int(fmt.Sprintf("%s_streams_inbound", prefix), baseLimit.StreamsInbound).
		Int(fmt.Sprintf("%s_streams_outbound", prefix), baseLimit.StreamsOutbound).
		Int(fmt.Sprintf("%s_conns", prefix), baseLimit.Conns).
		Int(fmt.Sprintf("%s_conns_inbound", prefix), baseLimit.ConnsInbound).
		Int(fmt.Sprintf("%s_conns_outbound", prefix), baseLimit.ConnsOutbound).
		Int(fmt.Sprintf("%s_file_descriptors", prefix), baseLimit.FD).
		Int64(fmt.Sprintf("%s_memory", prefix), baseLimit.Memory).Logger()
}
