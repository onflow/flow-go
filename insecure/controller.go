package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

// EgressController defines part of the behavior of a corruptible networking layer that controls outbound traffic of its engines.
// By the outbound traffic, we mean the traffic from engine to networking layer that passes through conduits, i.e., egress traffic of the engine.
type EgressController interface {
	// HandleOutgoingEvent sends an outgoing event (of an engine) to the corruptible networking layer.
	HandleOutgoingEvent(interface{}, channels.Channel, Protocol, uint32, ...flow.Identifier) error

	// EngineClosingChannel informs the corruptible networking layer that the corresponding engine of the given channel is not going to
	// use it anymore, hence the channel can be closed.
	EngineClosingChannel(channels.Channel) error
}

// IngressController defines part of behavior of a corrupted networking layer that controls the inbound traffic of
// the engines registered to it.
// By the inbound traffic, we mean the traffic from networking layer to the engine that carries on the
// messages from remote nodes to this engine.
type IngressController interface {
	// HandleIncomingEvent sends an incoming event (to an engine) to the corrupted networking layer to process.
	// Boolean return type represents whether attacker is registered with the corrupted network.
	// Returns true if it is, false otherwise.
	HandleIncomingEvent(interface{}, channels.Channel, flow.Identifier) bool
}
