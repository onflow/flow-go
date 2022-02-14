package corruptible

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type ConduitFactory struct {
	codec    network.Codec
	myId     flow.Identifier
	adapter  network.Adapter
	attacker AttackerClient
}

func NewCorruptibleConduitFactory(myId flow.Identifier, codec network.Codec) *ConduitFactory {
	return &ConduitFactory{
		myId:  myId,
		codec: codec,
	}
}

// RegisterAdapter sets the Adapter component of the factory.
// The Adapter is a wrapper around the Network layer that only exposes the set of methods
// that are needed by a conduit.
func (c *ConduitFactory) RegisterAdapter(adapter network.Adapter) error {
	if c.adapter != nil {
		return fmt.Errorf("could not register a new network adapter, one already exists")
	}

	c.adapter = adapter

	return nil
}

// NewConduit creates a conduit on the specified channel.
// Prior to creating any conduit, the factory requires an Adapter to be registered with it.
func (c *ConduitFactory) NewConduit(ctx context.Context, channel network.Channel) (network.Conduit, error) {
	if c.adapter == nil {
		return nil, fmt.Errorf("could not create a new conduit, missing a registered network adapter")
	}

	child, cancel := context.WithCancel(ctx)

	return &Conduit{
		ctx:     child,
		cancel:  cancel,
		channel: channel,
		adapter: c.adapter,
	}, nil
}

func (c *ConduitFactory) ProcessAttackerMessage(ctx context.Context, in *Message, opts ...grpc.CallOption) (*ActionResponse, error) {

}

func (c *ConduitFactory) RegisterAttacker(ctx context.Context, in *AttackerRegisterMessage, opts ...grpc.CallOption) (*ActionResponse, error) {

}
