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
	done    chan struct{}
	printed bool
}

func NewDiscovery(log zerolog.Logger, overlay middleware.Overlay, me flow.Identifier, done chan struct{}) *Discovery {
	d := &Discovery{overlay: overlay, log: log, me: me, done: done}
	return d
}

// Advertise is suppose to advertise this node's interest in a topic to a discovery service. However, we are not using
// a discovery service hence this function just returns a long duration to reduce the frequency with which libp2p calls it.
func (d *Discovery) Advertise(_ context.Context, _ string, _ ...discovery.Option) (time.Duration, error) {
	select {
	case <-d.done:
		return 0, fmt.Errorf("middleware stopped")
	default:
		return time.Hour, nil
	}
}

// FindPeers returns a channel providing all peers of the node. No parameters are needed as of now since the overlay.Identity
// provides all the information about the other nodes.
func (d *Discovery) FindPeers(_ context.Context, _ string, _ ...discovery.Option) (<-chan peer.AddrInfo, error) {

	// if middleware has been stopped, don't return any peers
	select {
	case <-d.done:
		emptyCh := make(chan peer.AddrInfo)
		close(emptyCh)
		return emptyCh, fmt.Errorf("middleware stopped")
	default:
	}

	// query the overlay to get all the other nodes that should be directly connected to this node for 1-k messaging
	ids, err := d.overlay.Topology()
	if err != nil {
		return nil, fmt.Errorf("failed to get ids: %w", err)
	}

	// remove self from list of nodes to avoid self dial
	delete(ids, d.me)

	// create a channel of peer.AddrInfo that needs to be returned
	ch := make(chan peer.AddrInfo, len(ids))

	directPeers := make([]flow.Identity, 0)

	// send each id to the channel after converting it to a peer.AddrInfo
	for _, id := range ids {
		// create a new NodeAddress
		ip, port, err := net.SplitHostPort(id.Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse address %s: %w", id.Address, err)
		}

		// convert the Flow key to a LibP2P key
		lkey, err := PublicKey(id.NetworkPubKey)
		if err != nil {
			return nil, fmt.Errorf("could not convert flow public key to libp2p public key: %v", err)
		}

		nodeAddress := NodeAddress{Name: id.NodeID.String(), IP: ip, Port: port, PubKey: lkey}
		addrInfo, err := GetPeerInfo(nodeAddress)
		if err != nil {
			return nil, fmt.Errorf(" invalid node address: %s, %w", nodeAddress.Name, err)
		}

		// add the address to the channel
		ch <- addrInfo

		directPeers = append(directPeers, id)
	}

	if !d.printed {
		d.log.Debug().Str("node", d.me.String()).Msgf("topology: %v", directPeers)
		d.printed = true
	}

	// close the channel as nothing else is going to be written to it
	close(ch)

	return ch, nil
}
