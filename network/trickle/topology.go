// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/trickle"
)

// Topology represents the state of the overlay network, such as which peers we
// are connected to, as well as which events they have seen.
type Topology interface {
	Up(nodeID model.Identifier)
	IsUp(nodeID model.Identifier) bool
	Down(nodeID model.Identifier)
	Seen(nodeID model.Identifier, hash []byte)
	HasSeen(nodeID model.Identifier, hash []byte) bool
	Count() uint
	Peers(filters ...trickle.PeerFilter) trickle.PeerList
}
