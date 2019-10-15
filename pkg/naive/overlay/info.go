// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

// Info holds the information we know about a peer on the network, such as his
// ID and what events he has seen.
type Info struct {
	ID   string
	Seen map[string]struct{}
}
