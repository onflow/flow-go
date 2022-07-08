package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type CorruptibleConduitFactory interface {
	network.ConduitFactory

	// SendOnFlowNetwork dispatches the given event to the networking layer of the node in order to be delivered
	// through the specified protocol to the target identifiers.
	SendOnFlowNetwork(interface{}, network.Channel, Protocol, uint, ...flow.Identifier) error

	// UnregisterChannel is called by the slave conduits of this factory to let it know that the corresponding engine of the
	// conduit is not going to use it anymore, so the channel can be closed safely.
	UnregisterChannel(network.Channel) error

	// RegisterEgressController sets the EgressController component of the factory.
	RegisterEgressController(EgressController) error
}
