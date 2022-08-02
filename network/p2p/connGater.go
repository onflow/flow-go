package p2p

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

var _ connmgr.ConnectionGater = (*ConnGater)(nil)

// ConnGater is the implementation of the libp2p connmgr.ConnectionGater interface
// It provides node allowlisting by libp2p peer.ID which is derived from the node public networking key
type ConnGater struct {
	sync.RWMutex
	log zerolog.Logger
	// interceptPeerDialFilters the list of peer filters that will be invoked in sync on outbound connections
	interceptPeerDialFilters PeerFilters
	// interceptSecuredFilters the list of peer filters that will be invoked in sync on inbound connections
	interceptSecuredFilters PeerFilters
}

type PeerFilter func(peer.ID) bool

type PeerFilters []PeerFilter

func NewConnGater(log zerolog.Logger, interceptPeerDialFilters, interceptSecuredFilters PeerFilters) *ConnGater {
	cg := &ConnGater{
		log:                      log,
		interceptPeerDialFilters: interceptPeerDialFilters,
		interceptSecuredFilters:  interceptSecuredFilters,
	}

	return cg
}

// InterceptPeerDial - a callback which allows or disallows outbound connection
func (c *ConnGater) InterceptPeerDial(p peer.ID) bool {
	for _, isAllowed := range c.interceptPeerDialFilters {
		if !isAllowed(p) {
			return false
		}
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

// InterceptSecured - a callback executed after the libp2p security handshake. It tests whether to accept or reject
// an inbound connection based on its peer id.
func (c *ConnGater) InterceptSecured(dir network.Direction, p peer.ID, addr network.ConnMultiaddrs) bool {
	switch dir {
	case network.DirInbound:
		allowed := c.interceptSecuredAllowed(p)
		if !allowed {
			// log the illegal connection attempt from the remote node
			c.log.Info().
				Str("node_id", p.Pretty()).
				Str("local_address", addr.LocalMultiaddr().String()).
				Str("remote_address", addr.RemoteMultiaddr().String()).
				Msg("rejected inbound connection")
		}
		return allowed
	default:
		// outbound connection should have been already blocked before this call
		return true
	}
}

// InterceptUpgraded Decision to continue or drop the connection should have been made before this call
func (c *ConnGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

// interceptSecuredAllowed invokes each intercept secure peer filter func and returns true if all filters return true.
func (c *ConnGater) interceptSecuredAllowed(p peer.ID) bool {
	for _, isAllowed := range c.interceptSecuredFilters {
		if !isAllowed(p) {
			return false
		}
	}

	return true
}
