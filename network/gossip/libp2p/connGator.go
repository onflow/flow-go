package libp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/dns"
)

var _ connmgr.ConnectionGater = (*connGater)(nil)

// connGater is the implementation of the libp2p connmgr.ConnectionGater interface
// It provides node allowlisting by libp2p peer.ID which is derived from the node public networking key
type connGater struct {
	peerIDAllowlist map[peer.ID]multiaddr.Multiaddr // the in-memory map of approved peer IDs and their corresponding multi-address
	addrsAllowList  map[string]struct{}             // the in-memory map of approved node multi-address (in string format)
	log             zerolog.Logger
	dnsMap          dns.DnsMap
}

func newConnGater(peerInfos []peer.AddrInfo, log zerolog.Logger, dnsMap dns.DnsMap) *connGater {
	cg := &connGater{
		log:    log,
		dnsMap: dnsMap,
	}
	cg.update(peerInfos)
	return cg
}

// update updates the peer ID map
func (c *connGater) update(peerInfos []peer.AddrInfo) {

	peerCount := len(peerInfos)

	// create a new address map
	addrs := make(map[string]struct{}, peerCount)

	// create a new peer.ID map
	peerIDs := make(map[peer.ID]multiaddr.Multiaddr, peerCount)

	// for each peer.AddrInfo create an entry in both maps
	for _, p := range peerInfos {

		if len(p.Addrs) <= 0 {
			log.Err(fmt.Errorf("no address found")).Str("peer", p.String())
			continue
		}

		addr := p.Addrs[0]
		addrs[addr.String()] = struct{}{}
		peerIDs[p.ID] = addr
	}

	// cache the new maps
	c.addrsAllowList = addrs
	c.peerIDAllowlist = peerIDs

	c.log.Info().Msg("approved list of peers updated")
}

// InterceptPeerDial - a calback which allows or disallows outbound connection
func (c *connGater) InterceptPeerDial(p peer.ID) bool {
	allow := c.validPeerID(p)
	if !allow {
		c.logRejection(network.DirOutbound, &p, nil)
	}
	return allow
}

// InterceptAddrDial is not used. Currently, allowlisting is only implemented by Peer IDs and not multi-addresses
func (c *connGater) InterceptAddrDial(p peer.ID, ma multiaddr.Multiaddr) bool {
	allow := c.validPeerIDAndAddress(p, ma)
	if !allow {
		c.logRejection(network.DirOutbound, &p, ma)
	}
	return allow
}

// InterceptAccept is not used. Currently, allowlisting is only implemented by Peer IDs and not multi-addresses
func (c *connGater) InterceptAccept(cm network.ConnMultiaddrs) bool {
	allow := c.validIPAddress(cm.RemoteMultiaddr())
	if !allow {
		c.logRejection(network.DirOutbound, nil, cm.RemoteMultiaddr())
	}
	return allow
}

// InterceptSecured - a callback executed after the libp2p security handshake. It tests whether to accept or reject
// an inbound connection based on its peer id.
func (c *connGater) InterceptSecured(dir network.Direction, p peer.ID, addr network.ConnMultiaddrs) bool {
	switch dir {
	case network.DirInbound:
		remoteAddr := addr.RemoteMultiaddr()
		allow := c.validPeerIDAndAddress(p, remoteAddr)
		if !allow {
			c.logRejection(dir, &p, remoteAddr)
		}
		return allow
	default:
		return true
	}
}

// Decision to continue or drop the connection should have been made before this call
func (c *connGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

func (c *connGater) validPeerID(p peer.ID) bool {
	_, ok := c.peerIDAllowlist[p]
	return ok
}

func (c *connGater) validIPAddress(remoteMA multiaddr.Multiaddr) bool {
	_, found := c.addrsAllowList[remoteMA.String()]
	if found {
		return true
	}
	hostname := c.dnsMap.ReverseLookUp(remoteMA)
	return hostname != nil
}

func (c *connGater) validPeerIDAndAddress(p peer.ID, targetMA multiaddr.Multiaddr) bool {
	ma, ok := c.peerIDAllowlist[p]
	if !ok {
		return false
	}

	var expectedMA multiaddr.Multiaddr
	if isHostname(targetMA) {
		expectedMA = c.dnsMap.LookUp(ma)
	} else {
		expectedMA = ma
	}

	return expectedMA != nil && targetMA.Equal(expectedMA)
}

func (c *connGater) logRejection(dir network.Direction, remotePeer *peer.ID, address multiaddr.Multiaddr) {
	log := c.log.With().Str("direction", dir.String()).Logger()
	if remotePeer != nil {
		log = log.With().Str("node_id", remotePeer.Pretty()).Logger()
	}
	if address != nil {
		log = log.With().Str("target", address.String()).Logger()
	}
	log.Info().Msg("connection rejected")
}

func isHostname(ma multiaddr.Multiaddr) bool {
	_, err := ma.ValueForProtocol(multiaddr.P_DNS4)
	return err != nil
}
