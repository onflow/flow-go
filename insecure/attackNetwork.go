package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network"
)

// AttackNetwork represents the networking interface that is available to the attacker for sending messages "through" corrupted nodes
// "to" the rest of the network.
type AttackNetwork interface {
	component.Component
	// RpcUnicastOnChannel enforces unicast-dissemination on the specified channel through a corrupted node.
	RpcUnicastOnChannel(flow.Identifier, network.Channel, interface{}, flow.Identifier) error

	// RpcPublishOnChannel enforces a publish-dissemination on the specified channel through a corrupted node.
	RpcPublishOnChannel(flow.Identifier, network.Channel, interface{}, ...flow.Identifier) error

	// RpcMulticastOnChannel enforces a multicast-dissemination on the specified channel through a corrupted node.
	RpcMulticastOnChannel(flow.Identifier, network.Channel, interface{}, uint32, ...flow.Identifier) error
}
