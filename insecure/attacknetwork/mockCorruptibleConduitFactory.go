package attacknetwork

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

type mockCorruptibleConduitFactory struct {
	component.Component
	cm                    *component.ComponentManager
	logger                zerolog.Logger
	codec                 network.Codec
	myId                  flow.Identifier
	adapter               network.Adapter
	attackerObserveClient insecure.Attacker_ObserveClient
	server                *grpc.Server // touch point of attack network to this factory.
	address               net.Addr
	ctx                   context.Context
}

func NewCorruptibleConduitFactory(
	logger zerolog.Logger,
	myId flow.Identifier,
	codec network.Codec,
	address string) *mockCorruptibleConduitFactory {

	factory := &ConduitFactory{
		myId:   myId,
		codec:  codec,
		logger: logger.With().Str("module", "corruptible-conduit-factory").Logger(),
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			factory.start(ctx, address)
			factory.ctx = ctx

			ready()

			<-ctx.Done()

			factory.stop()
		}).Build()

	factory.Component = cm
	factory.cm = cm

	return factory
}

// ServerAddress returns address of the gRPC server that is running by this corrupted conduit factory.
func (c *ConduitFactory) ServerAddress() string {
	return c.address.String()
}

func (c *ConduitFactory) start(ctx irrecoverable.SignalerContext, address string) {
	// starts up gRPC server of attack network at given address.
	s := grpc.NewServer()
	insecure.RegisterCorruptibleConduitFactoryServer(s, c)
	ln, err := net.Listen(networkingProtocolTCP, address)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
	}
	c.server = s
	c.address = ln.Addr()
	fmt.Println(c.address.String())

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		if err = s.Serve(ln); err != nil { // blocking call
			ctx.Throw(fmt.Errorf("could not bind factory to the tcp listener: %w", err))
		}
	}()

	wg.Wait()
}

// stop conducts the termination logic of the sub-modules of attack network.
func (c *mockCorruptibleConduitFactory) stop() {
	c.server.Stop()
}

// RegisterAdapter sets the Adapter component of the factory.
// The Adapter is a wrapper around the Network layer that only exposes the set of methods
// that are needed by a conduit.
func (c *mockCorruptibleConduitFactory) RegisterAdapter(adapter network.Adapter) error {
	if c.adapter != nil {
		return fmt.Errorf("could not register a new network adapter, one already exists")
	}

	c.adapter = adapter

	return nil
}

func (c *mockCorruptibleConduitFactory) ProcessAttackerMessage(stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-c.cm.ShutdownSignal():
			if c.attackerObserveClient != nil {
				_, err := c.attackerObserveClient.CloseAndRecv()
				if err != nil {
					c.logger.Fatal().Err(err).Msg("could not close processing stream from attacker")
					return err
				}
			}
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF || errors.Is(stream.Context().Err(), context.Canceled) {
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

// RegisterAttacker is a gRPC end-point for this conduit factory that lets an attacker register itself to it, so that the attacker can
// control it.
// Registering an attacker on a conduit is an exactly-once immutable operation, any second attempt after a successful registration returns an error.
func (c *mockCorruptibleConduitFactory) RegisterAttacker(ctx context.Context, in *insecure.AttackerRegisterMessage) (*empty.Empty, error) {
	select {
	case <-c.cm.ShutdownSignal():
		return nil, fmt.Errorf("conduit factory has been shut down")
	default:
		return &empty.Empty{}, c.registerAttacker(ctx, in.Address)
	}
}

// HandleIncomingEvent is called by the slave conduits of this factory to relay their incoming events.
// If there is an attacker registered to this factory, the event is dispatched to it.
// Otherwise, the factory follows the correct protocol path by sending the message down to the networking layer
// to deliver to its targets.
func (c *ConduitFactory) HandleIncomingEvent(
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
	targetNum uint32, targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := c.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	return &insecure.Message{
		ChannelID: channel.String(),
		OriginID:  c.myId[:],
		TargetNum: targetNum,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}, nil
}
