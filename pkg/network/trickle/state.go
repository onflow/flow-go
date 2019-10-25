// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import "encoding/hex"

// State represents the state of the overlay network, such as which peers we
// are connected to, as well as which events they have seen.
type State interface {
	Up(peerID string)
	Alive(peerID string) bool
	Seen(peerID string, hash []byte)
	Down(peerID string)
	Count() uint
	Peers(filters ...PeerFilter) PeerList
}

// PeerFilter is a function to filter peers by the information they hold.
type PeerFilter func(*Peer) bool

// Not negates the wrapped filter.
func Not(filter PeerFilter) PeerFilter {
	return func(p *Peer) bool {
		return !filter(p)
	}
}

// Seen filters for peers that have seen the event with the given ID.
func Seen(eventID []byte) PeerFilter {
	key := hex.EncodeToString(eventID)
	return func(p *Peer) bool {
		_, ok := p.Seen[key]
		return ok
	}
}

// SeenAll filters for peers that have seen all of the events with the given
// IDs.
func SeenAll(eventIDs ...[]byte) PeerFilter {
	keys := make([]string, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		key := hex.EncodeToString(eventID)
		keys = append(keys, key)
	}
	return func(p *Peer) bool {
		for _, key := range keys {
			_, ok := p.Seen[key]
			if !ok {
				return false
			}
		}
		return true
	}
}

// SeenOne filters for peers that have seen at least one of the events with the
// given IDs.
func SeenOne(eventIDs ...[]byte) PeerFilter {
	keys := make([]string, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		key := hex.EncodeToString(eventID)
		keys = append(keys, key)
	}
	return func(i *Peer) bool {
		for _, key := range keys {
			_, ok := i.Seen[key]
			if ok {
				return false
			}
		}
		return true
	}
}
