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

const refreshTopic = "refresh"

// Discovery implements the discovery.Discovery interface to provide libp2p a way to discover other nodes
type Discovery struct {
	ctx          context.Context
	log          zerolog.Logger
	overlay      middleware.Overlay
	me           flow.Identifier
	currentPeers flow.IdentityList
	libp2pNode   *P2PNode
}

func NewDiscovery(ctx context.Context, log zerolog.Logger, overlay middleware.Overlay, me flow.Identifier, libp2pNode *P2PNode) *Discovery {
	d := &Discovery{
		ctx:          ctx,
		overlay:      overlay,
		log:          log,
		me:           me,
		libp2pNode:   libp2pNode,
		currentPeers: make(flow.IdentityList, 0),
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

	// before returning, kick off a routine to disconnect peers no longer part of the node's identity list
	extraPeers := d.currentPeers.Filter(filter.Not(filter.In(ids)))
	cnt := extraPeers.Count()
	if cnt > 0 {
		d.log.Debug().Uint("extra_peer_count", cnt).Msg("disconnecting from extra peers")
		go d.disconnect(extraPeers)
	}

	// remember the new peers for next run
	d.currentPeers = ids

	return ch, nil
}

// disconnect disconnects the node from the extra peers
func (d *Discovery) disconnect(ids flow.IdentityList) {

	// unsubscribe from dummy topic used to initiate discovery
	defer func() {
		err := d.libp2pNode.UnSubscribe(refreshTopic)
		if err != nil {
			d.log.Err(err).Msg("failed to unsubscribe from discovery trigger topic")
		}
	}()

	for _, id := range ids {

		// check if the middleware context is still valid
		if d.ctx.Err() != nil {
			return
		}

		d.log.Debug().Str("peer", id.String()).Msg("disconnecting")

		nodeAddress, err := nodeAddressFromIdentity(*id)
		if err != nil {
			d.log.Err(err).Str("peer", id.String()).Msg("failed to disconnect")
			continue
		}

		err = d.libp2pNode.RemovePeer(d.ctx, nodeAddress)
		if err != nil {
			d.log.Err(err).Str("peer", id.String()).Msg("failed to disconnect")
		}
	}
}
