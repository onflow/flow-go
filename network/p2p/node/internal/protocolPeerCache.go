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

	"github.com/onflow/flow-go/network/p2p/logging"
)

// ProtocolPeerCache store a mapping from protocol ID to peers who support that protocol
type ProtocolPeerCache struct {
	protocolPeers map[protocol.ID]map[peer.ID]struct{}
	sync.RWMutex
}

func NewProtocolPeerCache(logger zerolog.Logger, h host.Host) (*ProtocolPeerCache, error) {
	sub, err := h.EventBus().
		Subscribe([]interface{}{new(event.EvtPeerIdentificationCompleted), new(event.EvtPeerProtocolsUpdated)})
	if err != nil {
		return nil, fmt.Errorf("could not subscribe to peer protocol update events: %w", err)
	}
	p := &ProtocolPeerCache{protocolPeers: make(map[protocol.ID]map[peer.ID]struct{})}
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
	for pid, peers := range p.protocolPeers {
		delete(peers, peerID)
		if len(peers) == 0 {
			delete(p.protocolPeers, pid)
		}
	}
}

func (p *ProtocolPeerCache) AddProtocols(peerID peer.ID, protocols []protocol.ID) {
	p.Lock()
	defer p.Unlock()
	for _, pid := range protocols {
		peers, ok := p.protocolPeers[pid]
		if !ok {
			peers = make(map[peer.ID]struct{})
			p.protocolPeers[pid] = peers
		}
		peers[peerID] = struct{}{}
	}
}

func (p *ProtocolPeerCache) RemoveProtocols(peerID peer.ID, protocols []protocol.ID) {
	p.Lock()
	defer p.Unlock()
	for _, pid := range protocols {
		peers := p.protocolPeers[pid]
		delete(peers, peerID)
		if len(peers) == 0 {
			delete(p.protocolPeers, pid)
		}
	}
}

func (p *ProtocolPeerCache) GetPeers(pid protocol.ID) map[peer.ID]struct{} {
	p.RLock()
	defer p.RUnlock()

	// it is not safe to return a reference to the map, so we make a copy
	peersCopy := make(map[peer.ID]struct{}, len(p.protocolPeers[pid]))
	for peerID := range p.protocolPeers[pid] {
		peersCopy[peerID] = struct{}{}
	}
	return peersCopy
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
