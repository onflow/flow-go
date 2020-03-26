// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package peer

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/model/trickle"
)

// Not negates the wrapped filter.
func Not(filter trickle.PeerFilter) trickle.PeerFilter {
	return func(p *trickle.Peer) bool {
		return !filter(p)
	}
}

// HasSeen filters for peers that have seen the event with the given ID.
func HasSeen(eventID []byte) trickle.PeerFilter {
	key := hex.EncodeToString(eventID)
	return func(p *trickle.Peer) bool {
		_, ok := p.Seen[key]
		return ok
	}
}

// HasSeenAllOf filters for peers that have seen all of the events with the given
// IDs.
func HasSeenAllOf(eventIDs ...[]byte) trickle.PeerFilter {
	keys := make([]string, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		key := hex.EncodeToString(eventID)
		keys = append(keys, key)
	}
	return func(p *trickle.Peer) bool {
		for _, key := range keys {
			_, ok := p.Seen[key]
			if !ok {
				return false
			}
		}
		return true
	}
}

// HasSeenOneOf filters for peers that have seen at least one of the events with the
// given IDs.
func HasSeenOneOf(eventIDs ...[]byte) trickle.PeerFilter {
	keys := make([]string, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		key := hex.EncodeToString(eventID)
		keys = append(keys, key)
	}
	return func(i *trickle.Peer) bool {
		for _, key := range keys {
			_, ok := i.Seen[key]
			if ok {
				return true
			}
		}
		return false
	}
}
