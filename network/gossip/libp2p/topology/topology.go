// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package topology

import (
	"encoding/hex"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/trickle"
)

// Topology implements a naive network state.
type Topology struct {
	sync.Mutex
	peers map[flow.Identifier]*trickle.Peer
}

// New creates a new naive state implementation for the overlay layer.
func New() (*Topology, error) {
	t := &Topology{
		peers: make(map[flow.Identifier]*trickle.Peer),
	}
	return t, nil
}

// Up will mark the current peer as being connected, initializing a new state
// for now.
func (t *Topology) Up(nodeID flow.Identifier) {
	t.Lock()
	defer t.Unlock()

	// return if the peer already exists
	if _, ok := t.peers[nodeID]; ok {
		return
	}

	t.peers[nodeID] = &trickle.Peer{
		ID:   nodeID,
		Seen: make(map[string]struct{}),
	}
}

// IsUp indicates whether the peer with given ID is currently connected.
func (t *Topology) IsUp(nodeID flow.Identifier) bool {
	t.Lock()
	defer t.Unlock()
	_, ok := t.peers[nodeID]
	return ok
}

// Down will mark the given peer as disconnected, dropping its state for now.
func (t *Topology) Down(nodeID flow.Identifier) {
	t.Lock()
	defer t.Unlock()
	delete(t.peers, nodeID)
}

// Seen marks an event as seen for the peer with given ID.
func (t *Topology) Seen(nodeID flow.Identifier, eventID []byte) {
	t.Lock()
	defer t.Unlock()
	peer, ok := t.peers[nodeID]
	if !ok {
		return
	}
	key := hex.EncodeToString(eventID)
	peer.Seen[key] = struct{}{}
}

// HasSeen indicates whether a given peer has seen a event with a given eventID
func (t *Topology) HasSeen(nodeID flow.Identifier, eventID []byte) bool {
	t.Lock()
	defer t.Unlock()
	peer, ok := t.peers[nodeID]
	if !ok {
		return false
	}
	key := hex.EncodeToString(eventID)
	_, ok = peer.Seen[key]
	return ok
}

// Count return the number of peers we are connected to.
func (t *Topology) Count() uint {
	t.Lock()
	defer t.Unlock()
	return uint(len(t.peers))
}

// Peers returns a filtered list of peers.
func (t *Topology) Peers(filters ...trickle.PeerFilter) trickle.PeerList {
	t.Lock()
	defer t.Unlock()
	var peers trickle.PeerList
Outer:
	for _, peer := range t.peers {
		for _, filter := range filters {
			ok := filter(peer)
			if !ok {
				continue Outer
			}
		}
		peers = append(peers, peer)
	}
	return peers
}
