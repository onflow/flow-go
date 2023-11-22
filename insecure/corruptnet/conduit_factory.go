package corruptnet

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/network/channels"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

const networkingProtocolTCP = "tcp"

// ConduitFactory implements a corrupt conduit factory, that creates corrupt conduits.
type ConduitFactory struct {
	logger           zerolog.Logger
	adapter          network.ConduitAdapter
	egressController insecure.EgressController
}

var _ insecure.CorruptConduitFactory = &ConduitFactory{}

func NewCorruptConduitFactory(logger zerolog.Logger, chainId flow.ChainID) *ConduitFactory {
	if chainId != flow.BftTestnet {
		panic("illegal chain id for using corrupt conduit factory")
	}

	factory := &ConduitFactory{
		logger: logger.With().Str("module", "corrupt-conduit-factory").Logger(),
	}

	return factory
}

// RegisterAdapter sets the ConduitAdapter component of the factory.
// The ConduitAdapter is a wrapper around the Network layer that only exposes the set of methods
// that are needed by a conduit.
func (c *ConduitFactory) RegisterAdapter(adapter network.ConduitAdapter) error {
	if c.adapter != nil {
		return fmt.Errorf("could not register a new network adapter, one already exists")
	}

	c.adapter = adapter

	return nil
}

// RegisterEgressController sets the EgressController component of the factory.
func (c *ConduitFactory) RegisterEgressController(controller insecure.EgressController) error {
	if c.egressController != nil {
		return fmt.Errorf("could not register a new egress controller, one already exists")
	}

	c.egressController = controller

	return nil
}

// NewConduit creates a conduit on the specified channel.
// Prior to creating any conduit, the factory requires an ConduitAdapter to be registered with it.
func (c *ConduitFactory) NewConduit(ctx context.Context, channel channels.Channel) (network.Conduit, error) {
	if c.adapter == nil {
		return nil, fmt.Errorf("could not create a new conduit, missing a registered network adapter")
	}

	if c.egressController == nil {
		return nil, fmt.Errorf("could not create a new conduit, missing a registered egress controller")
	}

	child, cancel := context.WithCancel(ctx)

	con := &Conduit{
		ctx:              child,
		cancel:           cancel,
		channel:          channel,
		egressController: c.egressController,
	}

	return con, nil
}

// UnregisterChannel is called by the secondary conduits of this factory to let it know that the corresponding engine of the
// conduit is not going to use it anymore, so the channel can be closed safely.
func (c *ConduitFactory) UnregisterChannel(channel channels.Channel) error {
	return c.adapter.UnRegisterChannel(channel)
}

// SendOnFlowNetwork dispatches the given event to the networking layer of the node in order to be delivered
// through the specified protocol to the target identifiers.
func (c *ConduitFactory) SendOnFlowNetwork(event interface{},
	channel channels.Channel,
	protocol insecure.Protocol,
	num uint, targetIds ...flow.Identifier) error {
	switch protocol {
	case insecure.Protocol_UNICAST:
		if len(targetIds) > 1 {
			return fmt.Errorf("more than one target ids for unicast: %v", targetIds)
		}
		return c.adapter.UnicastOnChannel(channel, event, targetIds[0])

	case insecure.Protocol_PUBLISH:
		return c.adapter.PublishOnChannel(channel, event, targetIds...)

	case insecure.Protocol_MULTICAST:
		return c.adapter.MulticastOnChannel(channel, event, num, targetIds...)
	default:
		return fmt.Errorf("unknown protocol for sending on network: %d", protocol)
	}
}
