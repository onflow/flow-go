// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

// State represents the state of the overlay network, such as which peers we
// are connected to, as well as which events they have seen.
type State interface {
	Init(id string)
	Alive(id string) bool
	Seen(id string, hash []byte)
	Drop(id string)
	Peers(filters ...FilterFunc) PeerList
}

// PeerList represents a list of peers.
type PeerList []*Info

// IDs returns the list of IDs for the peers in the list.
func (pl PeerList) IDs() []string {
	ids := make([]string, 0, len(pl))
	for _, p := range pl {
		ids = append(ids, p.ID)
	}
	return ids
}
