// Package gossip implements the functionality to epidemically dessiminate a message (e.g., transaction, collection, or block) within the network.
package gossip

import (
	"context"

	"github.com/golang/protobuf/proto"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
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

// GossipService is the interface of the network package and hence the networking streams with all
// other streams of the system. It defines the function call, the type of input arguments, and the output result
type GossipService interface {
	Gossip(ctx context.Context, request *shared.GossipMessage) ([]proto.Message, error)
}
