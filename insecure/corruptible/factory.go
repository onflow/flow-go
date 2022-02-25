package corruptible

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network"
)

// ConduitFactory implements a corruptible conduit factory, that creates corruptible conduits and acts as their master.
// A remote attacker can register itself to this conduit factory.
// Whenever any corruptible conduit generated by this factory receives an event from its engine, it relays the event to this
// factory, which in turn is relayed to the register attacker.
// The attacker can asynchronously dictate the conduit factory to send messages on behalf of the node this factory resides on.
type ConduitFactory struct {
	*component.ComponentManager
	logger                zerolog.Logger
	codec                 network.Codec
	myId                  flow.Identifier
	adapter               network.Adapter
	attackerObserveClient insecure.Attacker_ObserveClient
}

func NewCorruptibleConduitFactory(logger zerolog.Logger, chainId flow.ChainID, myId flow.Identifier, codec network.Codec) *ConduitFactory {
	if chainId != flow.BftTestnet {
		panic("illegal chain id for using corruptible conduit factory")
	}

	return &ConduitFactory{
		ComponentManager: component.NewComponentManagerBuilder().Build(),
		myId:             myId,
		codec:            codec,
		logger:           logger.With().Str("module", "corruptible-conduit-factory").Logger(),
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
		master:  c,
	}

	return con, nil
}

func (c *ConduitFactory) ProcessAttackerMessage(stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-c.ComponentManager.ShutdownSignal():
			if c.attackerObserveClient != nil {
				_, err := c.attackerObserveClient.CloseAndRecv()
				if err != nil {
					c.logger.Fatal().Err(err).Msg("could not close processing stream from attacker")
					return err
				}
			}
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				c.logger.Info().Msg("attacker closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				c.logger.Fatal().Err(err).Msg("could not read attacker's stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err := c.processAttackerMessage(msg); err != nil {
				c.logger.Fatal().Err(err).Msg("could not process attacker's message")
				return stream.SendAndClose(&empty.Empty{})
			}
		}

	}
}

// processAttackerMessage dispatches the attacker message on the Flow network on behalf of this node.
func (c *ConduitFactory) processAttackerMessage(msg *insecure.Message) error {
	event, err := c.codec.Decode(msg.Payload)
	if err != nil {
		return fmt.Errorf("could not decode message: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
	if err != nil {
		return fmt.Errorf("could not convert target ids from byte to identifiers: %w", err)
	}

	err = c.sendOnNetwork(event, network.Channel(msg.ChannelID), msg.Protocol, uint(msg.Targets), targetIds...)
	if err != nil {
		return fmt.Errorf("could not send attacker message to the network: %w", err)
	}

	return nil
}

// RegisterAttacker is a gRPC end-point for this conduit factory that lets an attacker register itself to it, so that the attacker can
// control it.
// Registering an attacker on a conduit is an exactly-once immutable operation, any second attempt after a successful registration returns an error.
func (c *ConduitFactory) RegisterAttacker(ctx context.Context, in *insecure.AttackerRegisterMessage) (*empty.Empty, error) {
	select {
	case <-c.ComponentManager.ShutdownSignal():
		return nil, fmt.Errorf("conduit factory has been shut down")
	default:
		return &empty.Empty{}, c.registerAttacker(ctx, in.Address)
	}
}

func (c *ConduitFactory) registerAttacker(ctx context.Context, address string) error {
	if c.attackerObserveClient != nil {
		c.logger.Error().Str("address", address).Msg("attacker double-register detected")
		return fmt.Errorf("illegal state: trying to register an attacker (%s) while one already exists", address)
	}

	clientConn, err := grpc.Dial(address)
	if err != nil {
		return fmt.Errorf("could not establish a client connection to attacker: %w", err)
	}

	attackerClient := insecure.NewAttackerClient(clientConn)
	c.attackerObserveClient, err = attackerClient.Observe(ctx)
	if err != nil {
		return fmt.Errorf("could not establish an observe stream to the attacker: %w", err)
	}

	c.logger.Info().Str("address", address).Msg("attacker registered successfully")

	return nil
}

// HandleIncomingEvent is called by the slave conduits of this factory to relay their incoming events.
// If there is an attacker registered to this factory, the event is dispatched to it.
// Otherwise, the factory follows the correct protocol path by sending the message down to the networking layer
// to deliver to its targets.
func (c *ConduitFactory) HandleIncomingEvent(
	ctx context.Context,
	event interface{},
	channel network.Channel,
	protocol insecure.Protocol,
	num uint32, targetIds ...flow.Identifier) error {

	if c.attackerObserveClient == nil {
		// no attacker yet registered, hence sending message on the network following the
		// correct expected behavior.
		return c.sendOnNetwork(event, channel, protocol, uint(num), targetIds...)
	}

	msg, err := c.eventToMessage(event, channel, protocol, num, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = c.attackerObserveClient.Send(msg)
	if err != nil {
		return fmt.Errorf("could not send message to attacker to observe: %w", err)
	}

	return nil
}

// EngineClosingChannel is called by the slave conduits of this factory to let it know that the corresponding engine of the
// conduit is not going to use it anymore, so the channel can be closed safely.
func (c *ConduitFactory) EngineClosingChannel(channel network.Channel) error {
	return c.adapter.UnRegisterChannel(channel)
}

// eventToMessage converts the given application layer event to a protobuf message that is meant to be sent to the attacker.
func (c *ConduitFactory) eventToMessage(
	event interface{},
	channel network.Channel,
	protocol insecure.Protocol,
	num uint32, targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := c.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	msgType := strings.TrimLeft(fmt.Sprintf("%T", event), "*")

	return &insecure.Message{
		ChannelID: channel.String(),
		OriginID:  c.myId[:],
		Targets:   num,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Type:      msgType,
		Protocol:  protocol,
	}, nil
}

// sendOnNetwork dispatches the given event to the networking layer of the node in order to be delivered
// through the specified protocol to the target identifiers.
func (c *ConduitFactory) sendOnNetwork(event interface{},
	channel network.Channel,
	protocol insecure.Protocol,
	num uint, targetIds ...flow.Identifier) error {
	switch protocol {
	case insecure.Protocol_UNICAST:
		if len(targetIds) > 1 {
			return fmt.Errorf("more than one target ids for unicast: %v", targetIds)
		}
		return c.adapter.UnicastOnChannel(channel, event, targetIds[0])

	case insecure.Protocol_PUBLISH:
		return c.adapter.PublishOnChannel(channel, event)

	case insecure.Protocol_MULTICAST:
		return c.adapter.MulticastOnChannel(channel, event, num, targetIds...)
	default:
		return fmt.Errorf("unknown protocol for sending on network: %d", protocol)
	}
}
