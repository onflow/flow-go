package insecure

import (
	"context"

	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// ConduitMaster defines part of the behavior of a corruptible conduit factory that controls the slave conduits it creates.
type ConduitMaster interface {
	// HandleIncomingEvent sends an incoming event to the conduit master to process.
	HandleIncomingEvent(context.Context, interface{}, network.Channel, proto.Protocol, uint32, ...flow.Identifier) error

	// EngineClosingChannel informs the conduit master that the corresponding engine of the given channel is not going to
	// use it anymore, hence the channel can be closed.
	EngineClosingChannel(network.Channel) error
}
