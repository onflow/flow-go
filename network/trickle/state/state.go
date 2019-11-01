// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package state

import (
	"encoding/hex"
	"sync"

	"github.com/dapperlabs/flow-go/network/trickle"
)

// State implements a naive network state.
type State struct {
	sync.Mutex
	peers map[string]*trickle.Peer
}

// New creates a new naive state implementation for the overlay layer.
func New() (*State, error) {
	s := &State{
		peers: make(map[string]*trickle.Peer),
	}
	return s, nil
}

// Up will mark the current peer as being connected, initializing a new state
// for now.
func (s *State) Up(peerID string) {
	s.Lock()
	defer s.Unlock()

	// return if the peer already exists
	if _, ok := s.peers[peerID]; ok {
		return
	}

	s.peers[peerID] = &trickle.Peer{
		ID:   peerID,
		Seen: make(map[string]struct{}),
	}
}

// Alive indicates whether the peer with given ID is currently connected.
func (s *State) Alive(peerID string) bool {
	s.Lock()
	defer s.Unlock()
	_, ok := s.peers[peerID]
	return ok
}

// Seen marks an event as seen for the peer with given ID.
func (s *State) Seen(peerID string, eventID []byte) {
	s.Lock()
	defer s.Unlock()
	peer, ok := s.peers[peerID]
	if !ok {
		return
	}
	key := hex.EncodeToString(eventID)
	peer.Seen[key] = struct{}{}
}

// PeerHaveSeen indicates whether a given peer has seen a event with a given eventID
func (s *State) PeerHaveSeen(peerID string, eventID []byte) bool {
	s.Lock()
	defer s.Unlock()
	peer, ok := s.peers[peerID]
	if !ok {
		return false
	}
	key := hex.EncodeToString(eventID)
	_, ok = peer.Seen[key]
	return ok
}

// Down will mark the given peer as disconnected, dropping its state for now.
func (s *State) Down(peerID string) {
	s.Lock()
	defer s.Unlock()
	delete(s.peers, peerID)
}

// Count return the number of peers we are connected to.
func (s *State) Count() uint {
	s.Lock()
	defer s.Unlock()
	return uint(len(s.peers))
}

// Peers returns a filtered list of peers.
func (s *State) Peers(filters ...trickle.PeerFilter) trickle.PeerList {
	s.Lock()
	defer s.Unlock()
	var peers trickle.PeerList
Outer:
	for _, peer := range s.peers {
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
