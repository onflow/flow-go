package connection

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

var _ connmgr.ConnectionGater = (*ConnGater)(nil)

// ConnGaterOption allow the connection gater to be configured with a list of PeerFilter funcs for a specific conn gater callback.
// In the current implementation of the ConnGater the following callbacks can be configured with peer filters.
// * InterceptPeerDial - peer filters can be configured with WithOnInterceptPeerDialFilters which will allow or disallow outbound connections.
// * InterceptSecured - peer filters can be configured with WithOnInterceptSecuredFilters which will allow or disallow inbound connections after libP2P security handshake.
type ConnGaterOption func(*ConnGater)

// WithOnInterceptPeerDialFilters sets peer filters for outbound connections.
func WithOnInterceptPeerDialFilters(filters []PeerFilter) ConnGaterOption {
	return func(c *ConnGater) {
		c.onInterceptPeerDialFilters = filters
	}
}

// WithOnInterceptSecuredFilters sets peer filters for inbound secured connections.
func WithOnInterceptSecuredFilters(filters []PeerFilter) ConnGaterOption {
	return func(c *ConnGater) {
		c.onInterceptSecuredFilters = filters
	}
}

// ConnGater is the implementation of the libp2p connmgr.ConnectionGater interface
// It provides node allowlisting by libp2p peer.ID which is derived from the node public networking key
type ConnGater struct {
	sync.RWMutex
	onInterceptPeerDialFilters []PeerFilter
	onInterceptSecuredFilters  []PeerFilter
	log                        zerolog.Logger
}

func NewConnGater(log zerolog.Logger, opts ...ConnGaterOption) *ConnGater {
	cg := &ConnGater{
		log: log.With().Str("component", "connection_gater").Logger(),
	}

	for _, opt := range opts {
		opt(cg)
	}

	return cg
}

// InterceptPeerDial - a callback which allows or disallows outbound connection
func (c *ConnGater) InterceptPeerDial(p peer.ID) bool {
	if len(c.onInterceptPeerDialFilters) == 0 {
		c.log.Debug().
			Str("peer_id", p.Pretty()).
			Msg("allowing outbound connection intercept peer dial has no peer filters set")
		return true
	}

	if err := c.peerIDPassesAllFilters(p, c.onInterceptPeerDialFilters); err != nil {
		// log the filtered outbound connection attempt
		c.log.Warn().
			Err(err).
			Str("peer_id", p.Pretty()).
			Msg("rejected outbound connection attempt")
		return false
	}

	return true
}

// InterceptAddrDial is not used. Currently, allowlisting is only implemented by Peer IDs and not multi-addresses
func (c *ConnGater) InterceptAddrDial(_ peer.ID, ma multiaddr.Multiaddr) bool {
	return true
}

// InterceptAccept is not used. Currently, allowlisting is only implemented by Peer IDs and not multi-addresses
func (c *ConnGater) InterceptAccept(cm network.ConnMultiaddrs) bool {
	return true
}

// InterceptSecured a callback executed after the libp2p security handshake. It tests whether to accept or reject
// an inbound connection based on its peer id.
func (c *ConnGater) InterceptSecured(dir network.Direction, p peer.ID, addr network.ConnMultiaddrs) bool {
	switch dir {
	case network.DirInbound:
		if len(c.onInterceptSecuredFilters) == 0 {
			c.log.Debug().
				Str("peer_id", p.Pretty()).
				Msg("allowing inbound connection intercept secured has no peer filters set")
			return true
		}

		if err := c.peerIDPassesAllFilters(p, c.onInterceptSecuredFilters); err != nil {
			// log the illegal connection attempt from the remote node
			c.log.Error().
				Err(err).
				Str("peer_id", p.Pretty()).
				Str("local_address", addr.LocalMultiaddr().String()).
				Str("remote_address", addr.RemoteMultiaddr().String()).
				Msg("rejected inbound connection")
			return false
		}

		return true
	default:
		// outbound connection should have been already blocked before this call
		return true
	}
}

// InterceptUpgraded decision to continue or drop the connection should have been made before this call
func (c *ConnGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

func (c *ConnGater) peerIDPassesAllFilters(p peer.ID, filters []PeerFilter) error {
	for _, allowed := range filters {
		if err := allowed(p); err != nil {
			return err
		}
	}

	return nil
}
