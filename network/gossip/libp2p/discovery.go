package libp2p

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// Discovery implements the discovery.Discovery interface to provide libp2p a way to discover other nodes
type Discovery struct {
	log     zerolog.Logger
	overlay middleware.Overlay
	me      flow.Identifier
}

func NewDiscovery(log zerolog.Logger, overlay middleware.Overlay, me flow.Identifier) *Discovery {
	d := &Discovery{overlay: overlay, log: log, me: me}
	return d
}

// Advertise is suppose to advertise this node's interest in a topic to a discovery service. However, we are not using
// a discovery service hence this function just returns a long duration to reduce the frequency with which libp2p calls it.
func (d *Discovery) Advertise(_ context.Context, _ string, _ ...discovery.Option) (time.Duration, error) {
	return time.Hour, nil
}

// FindPeers returns a channel providing all peers of the node. No parameters are needed as of now since the overlay.Identity
// provides all the information about the other nodes.
func (d *Discovery) FindPeers(_ context.Context, _ string, _ ...discovery.Option) (<-chan peer.AddrInfo, error) {

	// query the overlay to get all the other nodes
	ids, err := d.overlay.Identity()
	if err != nil {
		return nil, fmt.Errorf("failed to get ids: %w", err)
	}

	// remove self from list of nodes to avoid self dial
	delete(ids, d.me)

	// create a channel of peer.AddrInfo that needs to be returned
	ch := make(chan peer.AddrInfo, len(ids))

	// send each id to the channel after converting it to a peer.AddrInfo
	for _, id := range ids {
		// create a new NodeAddress
		ip, port, err := net.SplitHostPort(id.Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse address %s: %w", id.Address, err)
		}
		nodeAddress := NodeAddress{Name: id.NodeID.String(), IP: ip, Port: port}
		addrInfo, err := GetPeerInfo(nodeAddress)
		if err != nil {
			return nil, fmt.Errorf(" invalid node address: %s, %w", nodeAddress.Name, err)
		}

		// add the address to the channel
		ch <- addrInfo
	}

	// close the channel as nothing else is going to be written to it
	close(ch)

	return ch, nil
}
