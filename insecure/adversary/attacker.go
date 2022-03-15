package adversary

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

const networkingProtocolTCP = "tcp"

// Attacker implements the adversarial domain that is orchestrating an attack through corrupted nodes.
type Attacker struct {
	component.Component
	cm                 *component.ComponentManager
	address            string
	server             *grpc.Server
	logger             zerolog.Logger
	orchestrator       insecure.AttackOrchestrator
	codec              network.Codec
	corruptedIds       flow.IdentityList
	corruptedNodes     map[flow.Identifier]insecure.CorruptedNodeConnection
	corruptedConnector insecure.CorruptedNodeConnector
}

func NewAttacker(
	logger zerolog.Logger,
	address string,
	codec network.Codec,
	orchestrator insecure.AttackOrchestrator,
	connector insecure.CorruptedNodeConnector,
	corruptedIds flow.IdentityList) (*Attacker, error) {

	attacker := &Attacker{
		orchestrator:       orchestrator,
		logger:             logger,
		codec:              codec,
		address:            address,
		corruptedConnector: connector,
		corruptedIds:       corruptedIds,
		corruptedNodes:     make(map[flow.Identifier]insecure.CorruptedNodeConnection),
	}

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// starts up gRPC server of attacker at given address.
			s := grpc.NewServer()
			insecure.RegisterAttackerServer(s, attacker)
			ln, err := net.Listen(networkingProtocolTCP, attacker.address)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
			}
			attacker.server = s

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				wg.Done()
				if err = s.Serve(ln); err != nil { // blocking call
					ctx.Throw(fmt.Errorf("could not bind attacker to the tcp listener: %w", err))
				}
			}()

			// waits till gRPC server starts serving.
			wg.Wait()
			ready()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			err := attacker.start(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not start attacker: %w", err))
			}

			ready()

			<-ctx.Done()
			err = attacker.stop()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not stop attacker: %w", err))
			}

		}).Build()

	attacker.Component = cm
	attacker.cm = cm

	return attacker, nil
}

// start triggers the sub-modules of attacker.
func (a *Attacker) start(ctx irrecoverable.SignalerContext) error {
	for _, corruptedId := range a.corruptedIds {
		corruptibleClient, err := a.corruptedConnector.Connect(ctx, corruptedId.NodeID)
		if err != nil {
			return fmt.Errorf("could not establish corruptible client to node %x: %w", corruptedId.NodeID, err)
		}
		a.corruptedNodes[corruptedId.NodeID] = corruptibleClient
	}

	a.orchestrator.WithAttackNetwork(a)

	return nil
}

// Stop stops the sub-modules of attacker.
func (a *Attacker) stop() error {
	// tears down connections to corruptible nodes.
	var errors *multierror.Error
	for _, connection := range a.corruptedNodes {
		err := connection.CloseConnection()

		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	// tears down the attack server itself.
	a.server.Stop()

	return errors.ErrorOrNil()
}

// Observe implements the gRPC interface of attacker that is exposed to the corrupted conduits.
// Instead of dispatching their messages to the networking layer of Flow, the conduits of corrupted nodes
// dispatch the outgoing messages to the attacker by calling the Observe method of it remotely.
func (a *Attacker) Observe(stream insecure.Attacker_ObserveServer) error {
	for {
		select {
		case <-a.cm.ShutdownSignal():
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				a.logger.Info().Msg("attacker closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				return fmt.Errorf("could not read corrupted node's stream: %w", err)
			}

			if err = a.processObservedMsg(msg); err != nil {
				a.logger.Fatal().Err(err).Msg("could not process message of corrupted node")
				return stream.SendAndClose(&empty.Empty{})
			}
		}
	}
}

// processObserveMsg processes incoming messages arrived from corruptible conduits by passing them
// to the orchestrator.
func (a *Attacker) processObservedMsg(message *insecure.Message) error {
	event, err := a.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode observed payload: %w", err)
	}

	sender, err := flow.ByteSliceToId(message.OriginID)
	if err != nil {
		return fmt.Errorf("could not convert origin id to flow identifier: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(message.TargetIDs)
	if err != nil {
		return fmt.Errorf("could not convert target ids to flow identifiers: %w", err)
	}

	err = a.orchestrator.HandleEventFromCorruptedNode(&insecure.Event{
		CorruptedId:       sender,
		Channel:           network.Channel(message.ChannelID),
		FlowProtocolEvent: event,
		Protocol:          message.Protocol,
		TargetNum:         message.Targets,
		TargetIds:         targetIds,
	})
	if err != nil {
		return fmt.Errorf("could not handle event by orchestrator: %w", err)
	}

	return nil
}

// RpcUnicastOnChannel enforces unicast-dissemination on the specified channel through a corrupted node.
func (a *Attacker) RpcUnicastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetId flow.Identifier) error {
	connection, ok := a.corruptedNodes[corruptedId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", corruptedId)
	}

	msg, err := a.eventToMessage(corruptedId, event, channel, insecure.Protocol_UNICAST, 0, targetId)
	if err != nil {
		return fmt.Errorf("could not convert event to unicast message: %w", err)
	}

	err = connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not sent unicast event to corrupted node: %w", err)
	}

	return nil
}

// RpcPublishOnChannel enforces a publish-dissemination on the specified channel through a corrupted node.
func (a *Attacker) RpcPublishOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{},
	targetIds ...flow.Identifier) error {
	connection, ok := a.corruptedNodes[corruptedId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", corruptedId)
	}

	msg, err := a.eventToMessage(corruptedId, event, channel, insecure.Protocol_PUBLISH, 0, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to publish message: %w", err)
	}

	err = connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not sent publish event to corrupted node: %w", err)
	}

	return nil
}

// RpcMulticastOnChannel enforces a multicast-dissemination on the specified channel through a corrupted node.
func (a *Attacker) RpcMulticastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, num uint32,
	targetIds ...flow.Identifier) error {
	connection, ok := a.corruptedNodes[corruptedId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", corruptedId)
	}

	msg, err := a.eventToMessage(corruptedId, event, channel, insecure.Protocol_MULTICAST, num, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to multicast message: %w", err)
	}

	err = connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not sent multicast event to corrupted node: %w", err)
	}

	return nil
}

// eventToMessage converts the given application layer event to a protobuf message that is meant to be sent to the corrupted node.
func (a *Attacker) eventToMessage(corruptedId flow.Identifier,
	event interface{},
	channel network.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := a.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	return &insecure.Message{
		ChannelID: channel.String(),
		OriginID:  corruptedId[:],
		Targets:   num,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}, nil
}
