package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type CorruptConduitFactory interface {
	network.ConduitFactory

	// SendOnFlowNetwork dispatches the given event to the networking layer of the node in order to be delivered
	// through the specified protocol to the target identifiers.
	SendOnFlowNetwork(interface{}, channels.Channel, Protocol, uint, ...flow.Identifier) error

	// UnregisterChannel is called by the secondary conduits of this factory to let it know that the corresponding engine of the
	// conduit is not going to use it anymore, so the channel can be closed safely.
	UnregisterChannel(channels.Channel) error

	// RegisterEgressController sets the EgressController component of the factory. All outgoing messages of the (secondary) conduits that
	// this factory creates are forwarded to the EgressController instead of being dispatched on the Flow network.
	RegisterEgressController(EgressController) error
}
