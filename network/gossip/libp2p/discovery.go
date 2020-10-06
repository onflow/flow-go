package libp2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/middleware"
)

// Discovery implements the discovery.Discovery interface to provide libp2p a way to discover other nodes
type Discovery struct {
	ctx     context.Context
	log     zerolog.Logger
	overlay middleware.Overlay
	me      flow.Identifier
}

func NewDiscovery(ctx context.Context, log zerolog.Logger, overlay middleware.Overlay, me flow.Identifier) *Discovery {
	d := &Discovery{
		ctx:     ctx,
		overlay: overlay,
		log:     log,
		me:      me,
	}
	return d
}

// Advertise is suppose to advertise this node's interest in a topic to a discovery service. However, we are not using
// a discovery service hence this function just returns a long duration to reduce the frequency with which libp2p calls it.
func (d *Discovery) Advertise(ctx context.Context, _ string, _ ...discovery.Option) (time.Duration, error) {
	err := ctx.Err()
	if err != nil {
		return 0, err
	}
	return time.Hour, nil
}

// FindPeers returns a channel providing all peers of the node. No parameters are needed as of now since the overlay.Identity
// provides all the information about the other nodes.
func (d *Discovery) FindPeers(ctx context.Context, topic string, _ ...discovery.Option) (<-chan peer.AddrInfo, error) {

	d.log.Debug().Str("topic", topic).Msg("initiating peer discovery")

	err := ctx.Err()
	if err != nil {
		emptyCh := make(chan peer.AddrInfo)
		close(emptyCh)
		return emptyCh, err
	}
	// call the callback to get all the other nodes that should be directly connected to this node for 1-k messaging
	ids, err := d.overlay.Topology()
	if err != nil {
		return nil, fmt.Errorf("failed to get ids: %w", err)
	}

	// remove self from list of nodes to avoid self dial
	ids = ids.Filter(filter.Not(filter.HasNodeID(d.me)))

	// create a channel of peer.AddrInfo that needs to be returned
	ch := make(chan peer.AddrInfo, len(ids))

	// send each id to the channel after converting it to a peer.AddrInfo
	for _, id := range ids {

		nodeAddress, err := nodeAddressFromIdentity(*id)
		if err != nil {
			return nil, fmt.Errorf(" invalid node address: %s, %w", id.String(), err)
		}

		addrInfo, err := GetPeerInfo(nodeAddress)
		if err != nil {
			return nil, fmt.Errorf(" invalid node address: %s, %w", id.String(), err)
		}

		// add the address to the channel
		ch <- addrInfo
	}

	// close the channel as nothing else is going to be written to it
	close(ch)

	return ch, nil
}
