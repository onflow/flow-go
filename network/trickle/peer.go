// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

// Peer holds the information we know about a peer on the network, such as his
// ID and what events he has seen.
type Peer struct {
	ID   string
	Seen map[string]struct{}
}

// PeerList represents a list of peers.
type PeerList []*Peer

// IDs returns the list of IDs for the peers in the list.
func (pl PeerList) IDs() []string {
	peerIDs := make([]string, 0, len(pl))
	for _, p := range pl {
		peerIDs = append(peerIDs, p.ID)
	}
	return peerIDs
}
