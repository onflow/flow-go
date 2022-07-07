package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// EgressController defines part of the behavior of a corruptible conduit factory that controls outbound traffic of the conduits it creates.
// By the outbound traffic, we mean the traffic from conduit to networking layer, i.e., egress traffic of the engine.
type EgressController interface {
	// HandleOutgoingEvent sends an outgoing event (of an engine) to the conduit factory to process.
	HandleOutgoingEvent(interface{}, network.Channel, Protocol, uint32, ...flow.Identifier) error

	// EngineClosingChannel informs the conduit factory that the corresponding engine of the given channel is not going to
	// use it anymore, hence the channel can be closed.
	EngineClosingChannel(network.Channel) error
}

// IngressController defines part of behavior of a corrupted networking layer that controls the inbound traffic of
// the engines registered to it.
// By the inbound traffic, we mean the traffic from networking layer to the engine that carries on the
// messages from remote nodes to this engine.
type IngressController interface {
	// HandleIncomingEvent sends an incoming event (to an engine) to the corrupted networking layer to process.
	HandleIncomingEvent(network.Channel, flow.Identifier, interface{})
}
