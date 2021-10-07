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
	peerIDAllowlist map[peer.ID]struct{} // the in-memory map of approved peer IDs
	log             zerolog.Logger
}

func NewConnGater(log zerolog.Logger) *ConnGater {
	cg := &ConnGater{
		log: log,
	}
	return cg
}

// update updates the peer ID map
func (c *ConnGater) update(pids peer.IDSlice) {

	// create a new peer.ID map
	peerIDs := make(map[peer.ID]struct{}, len(pids))

	// for each peer.AddrInfo, create an entry in the map for the peer.ID
	for _, pid := range pids {
		peerIDs[pid] = struct{}{}
	}

	// cache the new map
	c.Lock()
	c.peerIDAllowlist = peerIDs
	c.Unlock()

	c.log.Info().Msg("approved list of peers updated")
}

// InterceptPeerDial - a callback which allows or disallows outbound connection
func (c *ConnGater) InterceptPeerDial(p peer.ID) bool {
	return c.validPeerID(p)
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
		allowed := c.validPeerID(p)
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

// Decision to continue or drop the connection should have been made before this call
func (c *ConnGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

func (c *ConnGater) validPeerID(p peer.ID) bool {
	c.RLock()
	defer c.RUnlock()
	_, ok := c.peerIDAllowlist[p]
	return ok
}
