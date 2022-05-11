package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// ConduitController defines part of the behavior of a corruptible conduit factory that controls the conduits it creates.
type ConduitController interface {
	// HandleIncomingEvent sends an incoming event to the conduit factory to process.
	HandleIncomingEvent(interface{}, network.Channel, Protocol, uint32, ...flow.Identifier) error

	// EngineClosingChannel informs the conduit factory that the corresponding engine of the given channel is not going to
	// use it anymore, hence the channel can be closed.
	EngineClosingChannel(network.Channel) error
}
