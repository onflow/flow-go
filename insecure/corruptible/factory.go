package corruptible

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type ConduitFactory struct {
	codec    network.Codec
	myId     flow.Identifier
	adapter  network.Adapter
	attacker proto.AttackerClient
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

	con := &Conduit{
		ctx:     child,
		cancel:  cancel,
		channel: channel,
		adapter: c.adapter,
	}

	return con, nil
}

func (c *ConduitFactory) ProcessAttackerMessage(_ context.Context, in *proto.Message, _ ...grpc.CallOption) (*empty.Empty, error) {
	event, err := c.codec.Decode(in.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not decode message: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(in.TargetIDs)
	if err != nil {
		return nil, fmt.Errorf("could not convert target ids from byte to identifiers: %w", err)
	}

	err = c.sendOnNetwork(event, network.Channel(in.ChannelID), in.Protocol, uint(in.Targets), targetIds...)
	if err != nil {
		return nil, fmt.Errorf("could not send attacker message to the network: %w", err)
	}

	return &empty.Empty{}, nil
}

// RegisterAttacker is a gRPC end-point for this conduit factory that lets an attacker register itself to it, so that the attacker can
// control it.
// Registering an attacker on a conduit is an exactly-once immutable operation, any second attempt after a successful registration returns an error.
func (c *ConduitFactory) RegisterAttacker(_ context.Context, in *proto.AttackerRegisterMessage, _ ...grpc.CallOption) (*empty.Empty, error) {
	if c.attacker != nil {
		return nil, fmt.Errorf("illegal state: trying to register an attacker (%s) while one already exists", in.Address)
	}

	clientConn, err := grpc.Dial(in.Address)
	if err != nil {
		return nil, fmt.Errorf("could not establish a client connection to attacker: %w", err)
	}

	c.attacker = proto.NewAttackerClient(clientConn)

	return &empty.Empty{}, nil
}

// Observe sends the given event to the registered attacker, if one exists.
// Boolean return value determines whether the event has successfully dispatched to the attacker, or not.
func (c *ConduitFactory) Observe(
	ctx context.Context,
	event interface{},
	channel network.Channel,
	protocol proto.Protocol,
	num uint32, targetIds ...flow.Identifier) (bool, error) {

	if c.attacker == nil {
		// no attacker yet registered
		return false, nil
	}

	msg, err := c.eventToMessage(event, channel, protocol, num, targetIds...)
	if err != nil {
		return false, fmt.Errorf("could not convert event to message: %w", err)
	}

	_, err = c.attacker.Observe(ctx, msg)
	if err != nil {
		return false, fmt.Errorf("remote attacker could not observe message: %w", err)
	}

	return true, nil
}

// eventToMessage converts the given application layer event to a protobuf message that is meant to be sent to the attacker.
func (c *ConduitFactory) eventToMessage(
	event interface{},
	channel network.Channel,
	protocol proto.Protocol,
	num uint32, targetIds ...flow.Identifier) (*proto.Message, error) {

	payload, err := c.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	msgType := strings.TrimLeft(fmt.Sprintf("%T", event), "*")

	return &proto.Message{
		ChannelID: channel.String(),
		OriginID:  c.myId[:],
		Targets:   num,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Type:      msgType,
		Protocol:  protocol,
	}, nil
}

func (c *ConduitFactory) sendOnNetwork(event interface{},
	channel network.Channel,
	protocol proto.Protocol,
	num uint, targetIds ...flow.Identifier) error {
	switch protocol {
	case proto.Protocol_UNICAST:
		if len(targetIds) > 1 {
			return fmt.Errorf("illegal state: one target ids for unicast: %v", targetIds)
		}
		return c.adapter.UnicastOnChannel(channel, event, targetIds[0])

	case proto.Protocol_PUBLISH:
		return c.adapter.PublishOnChannel(channel, event)

	case proto.Protocol_MULTICAST:
		return c.adapter.MulticastOnChannel(channel, event, num, targetIds...)
	default:
		return fmt.Errorf("unknown protocol for sending on network: %d", protocol)
	}
}
