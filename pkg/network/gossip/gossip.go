// Package gossip implements the functionality to epidemically dessiminate a message (e.g., transaction, collection, or block) within the network.
package gossip

import (
	"context"

	"github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/golang/protobuf/proto"
)

// Defines mode of the gossip based on the set of recipients
type GossipMode int

// The legitimate values for the GossipMode
const (
	ONE_TO_ONE GossipMode = iota
	ONE_TO_MANY
	ONE_TO_ALL
)

func (gm GossipMode) String() string {
	return [...]string{"one_to_one", "one_to_many", "one_to_all"}[gm]
}

// // Message models a single message that is supposed to get exchanged by the gossip network
// type Message struct {
// 	Payload    proto.Message // The payload of the message, i.e., the data to be exchanged
// 	Method     string        // Name of RPC method to be invoked on the message
// 	Recipients []string      // Address of the recipients to which the message should be delivered
// 	Sender     string        // Address of the sender of this message
// 	Path       []string      // Address of the nodes that this message visited so far
// 	TTL        int           // The time to live of the message, i.e., maximum number of hops it should be gossiped
// }

// GossipService is the interface of the network package and hence the networking streams with all
// other streams of the system. It defines the function call, the type of input arguments, and the output result
type GossipService interface {
	Gossip(ctx context.Context, request *shared.GossipMessage) (proto.Message, error)
}
