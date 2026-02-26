package internal

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
)

// ProtocolPeerCache store a mapping from protocol ID to peers who support that protocol
type ProtocolPeerCache struct {
	protocolPeers map[protocol.ID]map[peer.ID]struct{}
	sync.RWMutex
}

// NewProtocolPeerCache creates a new ProtocolPeerCache instance using the given host and supported protocols
// Only protocols passed in the protocols list will be tracked
func NewProtocolPeerCache(logger zerolog.Logger, h host.Host, protocols []protocol.ID) (*ProtocolPeerCache, error) {
	protocolPeers := make(map[protocol.ID]map[peer.ID]struct{})
	for _, pid := range protocols {
		protocolPeers[pid] = make(map[peer.ID]struct{})
	}
	p := &ProtocolPeerCache{protocolPeers: protocolPeers}

	// If no protocols are passed, this is a noop cache
	if len(protocols) == 0 {
		return p, nil
	}

	sub, err := h.EventBus().
		Subscribe([]any{new(event.EvtPeerIdentificationCompleted), new(event.EvtPeerProtocolsUpdated)})
	if err != nil {
		return nil, fmt.Errorf("could not subscribe to peer protocol update events: %w", err)
	}

	h.Network().Notify(&libp2pnet.NotifyBundle{
		DisconnectedF: func(n libp2pnet.Network, c libp2pnet.Conn) {
			peer := c.RemotePeer()
			if len(n.ConnsToPeer(peer)) == 0 {
				p.RemovePeer(peer)
			}
		},
	})
	go p.consumeSubscription(logger, h, sub)

	return p, nil
}

func (p *ProtocolPeerCache) RemovePeer(peerID peer.ID) {
	p.Lock()
	defer p.Unlock()
	for _, peers := range p.protocolPeers {
		delete(peers, peerID)
	}
}

func (p *ProtocolPeerCache) AddProtocols(peerID peer.ID, protocols []protocol.ID) {
	p.Lock()
	defer p.Unlock()
	for _, pid := range protocols {
		if peers, ok := p.protocolPeers[pid]; ok {
			peers[peerID] = struct{}{}
		}
	}
}

func (p *ProtocolPeerCache) RemoveProtocols(peerID peer.ID, protocols []protocol.ID) {
	p.Lock()
	defer p.Unlock()
	for _, pid := range protocols {
		if peers, ok := p.protocolPeers[pid]; ok {
			delete(peers, peerID)
		}
	}
}

func (p *ProtocolPeerCache) GetPeers(pid protocol.ID) peer.IDSlice {
	p.RLock()
	defer p.RUnlock()

	peers, ok := p.protocolPeers[pid]
	if !ok {
		return peer.IDSlice{}
	}

	return maps.Keys(peers)
}

func (p *ProtocolPeerCache) consumeSubscription(logger zerolog.Logger, h host.Host, sub event.Subscription) {
	defer sub.Close()
	logger.Debug().Msg("starting peer protocol event subscription loop")
	for e := range sub.Out() {
		logger.Debug().Interface("event", e).Msg("received new peer protocol event")
		switch evt := e.(type) {
		case event.EvtPeerIdentificationCompleted:
			protocols, err := h.Peerstore().GetProtocols(evt.Peer)
			if err != nil {
				logger.Err(err).Str("peer_id", p2plogging.PeerId(evt.Peer)).Msg("failed to get protocols for peer")
				continue
			}
			p.AddProtocols(evt.Peer, protocols)
		case event.EvtPeerProtocolsUpdated:
			p.AddProtocols(evt.Peer, evt.Added)
			p.RemoveProtocols(evt.Peer, evt.Removed)
		}
	}
	logger.Debug().Msg("exiting peer protocol event subscription loop")
}
