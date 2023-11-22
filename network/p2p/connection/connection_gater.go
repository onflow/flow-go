package connection

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/utils/logging"
)

var _ p2p.ConnectionGater = (*ConnGater)(nil)

// ConnGaterOption allow the connection gater to be configured with a list of PeerFilter funcs for a specific conn gater callback.
// In the current implementation of the ConnGater the following callbacks can be configured with peer filters.
// * InterceptPeerDial - peer filters can be configured with WithOnInterceptPeerDialFilters which will allow or disallow outbound connections.
// * InterceptSecured - peer filters can be configured with WithOnInterceptSecuredFilters which will allow or disallow inbound connections after libP2P security handshake.
type ConnGaterOption func(*ConnGater)

// WithOnInterceptPeerDialFilters sets peer filters for outbound connections.
func WithOnInterceptPeerDialFilters(filters []p2p.PeerFilter) ConnGaterOption {
	return func(c *ConnGater) {
		c.onInterceptPeerDialFilters = filters
	}
}

// WithOnInterceptSecuredFilters sets peer filters for inbound secured connections.
func WithOnInterceptSecuredFilters(filters []p2p.PeerFilter) ConnGaterOption {
	return func(c *ConnGater) {
		c.onInterceptSecuredFilters = filters
	}
}

// ConnGater is the implementation of the libp2p connmgr.ConnectionGater interface
// It provides node allowlisting by libp2p peer.ID which is derived from the node public networking key
type ConnGater struct {
	sync.RWMutex
	onInterceptPeerDialFilters []p2p.PeerFilter
	onInterceptSecuredFilters  []p2p.PeerFilter

	// disallowListOracle is consulted upon every incoming or outgoing connection attempt, and the connection is only
	// allowed if the remote peer is not on the disallow list.
	// A ConnGater must have a disallowListOracle set, and if one is not set the ConnGater will panic.
	disallowListOracle p2p.DisallowListOracle

	// identityProvider provides the identity of a node given its peer ID for logging purposes only.
	// It is not used for allowlisting or filtering. We use the onInterceptPeerDialFilters and onInterceptSecuredFilters
	// to determine if a node should be allowed to connect.
	identityProvider module.IdentityProvider
	log              zerolog.Logger
}

func NewConnGater(log zerolog.Logger, identityProvider module.IdentityProvider, opts ...ConnGaterOption) *ConnGater {
	cg := &ConnGater{
		log:              log.With().Str("component", "connection_gater").Logger(),
		identityProvider: identityProvider,
	}

	for _, opt := range opts {
		opt(cg)
	}

	return cg
}

// InterceptPeerDial - a callback which allows or disallows outbound connection
func (c *ConnGater) InterceptPeerDial(p peer.ID) bool {
	lg := c.log.With().Str("peer_id", p2plogging.PeerId(p)).Logger()

	disallowListCauses, disallowListed := c.disallowListOracle.IsDisallowListed(p)
	if disallowListed {
		lg.Warn().
			Str("disallow_list_causes", fmt.Sprintf("%v", disallowListCauses)).
			Msg("outbound connection attempt to disallow listed peer is rejected")
		return false
	}

	if len(c.onInterceptPeerDialFilters) == 0 {
		lg.Warn().
			Msg("outbound connection established with no intercept peer dial filters")
		return true
	}

	identity, ok := c.identityProvider.ByPeerID(p)
	if !ok {
		lg = lg.With().
			Str("remote_node_id", "unknown").
			Str("role", "unknown").
			Logger()
	} else {
		lg = lg.With().
			Hex("remote_node_id", logging.ID(identity.NodeID)).
			Str("role", identity.Role.String()).
			Logger()
	}

	if err := c.peerIDPassesAllFilters(p, c.onInterceptPeerDialFilters); err != nil {
		// log the filtered outbound connection attempt
		lg.Warn().
			Err(err).
			Msg("rejected outbound connection attempt")
		return false
	}

	lg.Debug().Msg("outbound connection established")
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
		lg := c.log.With().
			Str("peer_id", p2plogging.PeerId(p)).
			Str("remote_address", addr.RemoteMultiaddr().String()).
			Logger()

		disallowListCauses, disallowListed := c.disallowListOracle.IsDisallowListed(p)
		if disallowListed {
			lg.Warn().
				Str("disallow_list_causes", fmt.Sprintf("%v", disallowListCauses)).
				Msg("inbound connection attempt to disallow listed peer is rejected")
			return false
		}

		if len(c.onInterceptSecuredFilters) == 0 {
			lg.Warn().Msg("inbound connection established with no intercept secured filters")
			return true
		}

		identity, ok := c.identityProvider.ByPeerID(p)
		if !ok {
			lg = lg.With().
				Str("remote_node_id", "unknown").
				Str("role", "unknown").
				Logger()
		} else {
			lg = lg.With().
				Hex("remote_node_id", logging.ID(identity.NodeID)).
				Str("role", identity.Role.String()).
				Logger()
		}

		if err := c.peerIDPassesAllFilters(p, c.onInterceptSecuredFilters); err != nil {
			// log the illegal connection attempt from the remote node
			lg.Error().
				Err(err).
				Str("local_address", addr.LocalMultiaddr().String()).
				Bool(logging.KeySuspicious, true).
				Msg("rejected inbound connection")
			return false
		}

		lg.Debug().Msg("inbound connection established")
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

func (c *ConnGater) peerIDPassesAllFilters(p peer.ID, filters []p2p.PeerFilter) error {
	for _, allowed := range filters {
		if err := allowed(p); err != nil {
			return err
		}
	}

	return nil
}

// SetDisallowListOracle sets the disallow list oracle for the connection gater.
// If one is set, the oracle is consulted upon every incoming or outgoing connection attempt, and
// the connection is only allowed if the remote peer is not on the disallow list.
// In Flow blockchain, it is not optional to dismiss the disallow list oracle, and if one is not set
// the connection gater will panic.
// Also, it follows a dependency injection pattern and does not allow to set the disallow list oracle more than once,
// any subsequent calls to this method will result in a panic.
// Args:
//
//	oracle: the disallow list oracle to set.
//
// Returns:
//
//	none
//
// Panics:
//
//	if the disallow list oracle is already set.
func (c *ConnGater) SetDisallowListOracle(oracle p2p.DisallowListOracle) {
	if c.disallowListOracle != nil {
		panic("disallow list oracle already set")
	}
	c.disallowListOracle = oracle
}
