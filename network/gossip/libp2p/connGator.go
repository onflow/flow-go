package libp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

var _ connmgr.ConnectionGater = (*connGater)(nil)

type connGater struct {
	peerIDWhitelist  map[peer.ID]struct{}
	ipAddrsWhitelist map[string]struct{}
	log              zerolog.Logger
}

func newConnGater(peerInfos []peer.AddrInfo, log zerolog.Logger) (*connGater, error) {
	cg := &connGater{
		log: log,
	}
	err := cg.update(peerInfos)
	return cg, err
}

func (c *connGater) update(peerInfos []peer.AddrInfo) error {

	peerIDs := make(map[peer.ID]struct{}, len(peerInfos))
	ipAddrs := make(map[string]struct{}, len(peerInfos))

	for _, p := range peerInfos {
		peerIDs[p.ID] = struct{}{}
		for _, ma := range p.Addrs {
			ip, err := ma.ValueForProtocol(multiaddr.P_IP4)
			if err != nil {
				return fmt.Errorf("failed to get IPv4 address from %s", ma.String())
			}
			ipAddrs[ip] = struct{}{}
		}
	}

	c.peerIDWhitelist = peerIDs
	c.ipAddrsWhitelist = ipAddrs
	return nil
}

func (c *connGater) InterceptPeerDial(p peer.ID) bool {
	return c.validPeerID(p)
}

func (c *connGater) InterceptAddrDial(p peer.ID, _ multiaddr.Multiaddr) bool {
	return c.validPeerID(p)
}

func (c *connGater) InterceptAccept(cm network.ConnMultiaddrs) bool {
	remoteAddr := cm.RemoteMultiaddr()
	return c.validMultiAddr(remoteAddr)
}

func (c *connGater) InterceptSecured(_ network.Direction, p peer.ID, _ network.ConnMultiaddrs) bool {
	return c.validPeerID(p) // ip address should have been already verified earlier in InterceptAccept
}

func (c *connGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

func (c *connGater) validPeerID(p peer.ID) bool {
	_, ok := c.peerIDWhitelist[p]
	return ok
}

func (c *connGater) validMultiAddr(addr multiaddr.Multiaddr) bool {
	ip, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		c.log.Err(err).Str("multiaddress", addr.String()).Msg("failed to determine IP address of client")
		return false
	}

	_, found := c.ipAddrsWhitelist[ip]
	return found
}
