package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type CorruptibleConduitFactory interface {
	network.Conduit

	// SendOnFlowNetwork dispatches the given event to the networking layer of the node in order to be delivered
	// through the specified protocol to the target identifiers.
	SendOnFlowNetwork(interface{}, network.Channel, Protocol, uint, ...flow.Identifier) error
}
