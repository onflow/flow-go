// Package gossip implements the functionality to epidemically dessiminate a message (e.g., transaction, collection, or block) within the network.
package gossip

import (
	"context"

	"github.com/dapperlabs/flow-go/network/gossip/registry"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// Mode defines mode of the gossip based on the set of recipients
type Mode int

// The legitimate values for the a gossip Mode
const (
	ModeOneToOne Mode = iota
	ModeOneToMany
	ModeOneToAll
)

func (m Mode) String() string {
	return [...]string{"ModeOneToOne", "ModeOneToMany", "ModeOneToAll"}[m]
}

// Service is the interface of the network package and hence the networking streams with all
// other streams of the system. It defines the function call, the type of input arguments, and the output result
type Service interface {
	Gossip(ctx context.Context, payload []byte, recipients []string, method registry.MessageType) ([]*messages.GossipReply, error)
}
