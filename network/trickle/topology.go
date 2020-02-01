// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package trickle

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/trickle"
)

// Deprecated: completely removed in new libp2p package architecture
//
// Topology represents the state of the overlay network, such as which peers we
// are connected to, as well as which events they have seen.
type Topology interface {
	Up(nodeID flow.Identifier)
	IsUp(nodeID flow.Identifier) bool
	Down(nodeID flow.Identifier)
	Seen(nodeID flow.Identifier, hash []byte)
	HasSeen(nodeID flow.Identifier, hash []byte) bool
	Count() uint
	Peers(filters ...trickle.PeerFilter) trickle.PeerList
}
