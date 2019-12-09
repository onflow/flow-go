// libp2p package encapsulates the libp2p library
package libp2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

// NodeAddress is used to define a libp2p node
type NodeAddress struct {
	// name is the friendly node name e.g. "node1" (not to be confused with the libp2p node id)
	name string
	ip   string
	port string
}

// P2PNode manages the the libp2p node.
type P2PNode struct {
	name       string
	libP2PHost host.Host
}

// Start starts a libp2p node on the given address.
func (l *P2PNode) Start(n NodeAddress) (err error) {
	l.name = n.name
	sourceMultiAddr, err := GetLocationMultiaddr(n)
	if err != nil {
		return err
	}

	key, err := GetPublicKey(n.name)
	if err != nil {
		return err
	}

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.NoSecurity,
		libp2p.Identity(key),
	)

	l.libP2PHost = host
	return err
}

// Stop stops the libp2p node.
func (l *P2PNode) Stop() (err error) {
	return l.libP2PHost.Network().Close()
}

// AddPeers adds other nodes as peers to this node
func (l *P2PNode) AddPeers(peers []NodeAddress) error {
	for _, p := range peers {
		pInfo, err := GetPeerInfo(p)
		if err != nil {
			return err
		}

		// Add the destination's peer multiaddress in the peerstore.
		// This will be used during connection and stream creation by libp2p.
		l.libP2PHost.Peerstore().AddAddr(pInfo.ID, pInfo.Addrs[0], peerstore.PermanentAddrTTL)

		err = l.libP2PHost.Connect(context.Background(), pInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetPeerInfo generates the address of a Node/Peer given it's address in a deterministic and consistent way.
// Libp2p uses the hash of the public key of node as its id (https://docs.libp2p.io/reference/glossary/#multihash)
// Since the public key of a node may not be available to other nodes, for now a simple scheme of naming nodes can be
// used e.g. "node1, node2,... nodex" to helps nodes address each other.
// An MD5 hash of such of the node name is used as a seed to a deterministic crypto algorithm to generate the
// public key from which libp2p derives the node id
func GetPeerInfo(p NodeAddress) (peer.AddrInfo, error) {
	maddr, err := GetLocationMultiaddr(p)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	key, err := GetPublicKey(p.name)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	id, err := peer.IDFromPublicKey(key.GetPublic())
	if err != nil {
		return peer.AddrInfo{}, err
	}
	pInfo := peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{maddr}}
	return pInfo, err
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (l *P2PNode) GetIPPort() (ip string, port string) {
	for _, a := range l.libP2PHost.Network().ListenAddresses() {
		if ip, e := a.ValueForProtocol(multiaddr.P_IP4); e == nil {
			if p, e := a.ValueForProtocol(multiaddr.P_TCP); e == nil {
				return ip, p
			}
		}
	}
	return "", ""
}
