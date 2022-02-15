package corruptible

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type ConduitFactory struct {
	codec    network.Codec
	myId     flow.Identifier
	adapter  network.Adapter
	attacker AttackerClient
	conduits map[string]*Conduit
}

func NewCorruptibleConduitFactory(myId flow.Identifier, codec network.Codec) *ConduitFactory {
	return &ConduitFactory{
		myId:     myId,
		codec:    codec,
		conduits: make(map[string]*Conduit),
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

	con := &Conduit{
		ctx:     child,
		cancel:  cancel,
		channel: channel,
		adapter: c.adapter,
	}

	c.conduits[channel.String()] = con

	return con, nil
}

func (c *ConduitFactory) ProcessAttackerMessage(ctx context.Context, in *Message, opts ...grpc.CallOption) (*empty.Empty, error) {

}

// RegisterAttacker is a gRPC end-point for this conduit factory that lets an attacker register itself to it, so that the attacker can
// control it.
// Registering an attacker on a conduit is an exactly-once immutable operation, any second attempt after a successful registration returns an error.
func (c *ConduitFactory) RegisterAttacker(ctx context.Context, in *AttackerRegisterMessage, opts ...grpc.CallOption) (*empty.Empty, error) {
	if c.attacker != nil {
		return nil, fmt.Errorf("illegal state: trying to register an attacker (%s) while one already exists", in.Address)
	}

	clientConn, err := grpc.Dial(in.Address)
	if err != nil {
		return nil, fmt.Errorf("could not establish a client connection to attacker: %w", err)
	}

	c.attacker = NewAttackerClient(clientConn)

	return &empty.Empty{}, nil
}
