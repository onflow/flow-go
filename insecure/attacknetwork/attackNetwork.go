package attacknetwork

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// AttackNetworkFactory implements a middleware for mounting an attack orchestrator and empowering it to communicate with the corrupted nodes.
type AttackNetworkFactory struct {
	component.Component
	cm                   *component.ComponentManager
	orchestratorMutex    sync.Mutex // to ensure thread-safe calls into orchestrator.
	logger               zerolog.Logger
	orchestrator         insecure.AttackOrchestrator // the mounted orchestrator that implements certain attack logic.
	codec                network.Codec
	corruptedNodeIds     flow.IdentityList                                    // identity of the corrupted nodes
	corruptedConnections map[flow.Identifier]insecure.CorruptedNodeConnection // existing connections to the corrupted nodes.
	corruptedConnector   insecure.CorruptedNodeConnector                      // connection generator to corrupted nodes.
}

var _ insecure.AttackNetwork = &AttackNetwork{}

func NewAttackNetwork(
	logger zerolog.Logger,
	codec network.Codec,
	orchestrator insecure.AttackOrchestrator,
	connector insecure.CorruptedNodeConnector,
	corruptedNodeIds flow.IdentityList) (*AttackNetworkFactory, error) {

	attackNetwork := &AttackNetworkFactory{
		orchestrator:         orchestrator,
		logger:               logger,
		codec:                codec,
		corruptedConnector:   connector,
		corruptedNodeIds:     corruptedNodeIds,
		corruptedConnections: make(map[flow.Identifier]insecure.CorruptedNodeConnection),
	}

	connector.WithIncomingMessageHandler(attackNetwork.Observe)

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			err := attackNetwork.start(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not start attackNetwork: %w", err))
			}

			ready()

			<-ctx.Done()

			err = attackNetwork.stop()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not stop attackNetwork: %w", err))
			}
		}).Build()

	attackNetwork.Component = cm
	attackNetwork.cm = cm

	return attackNetwork, nil
}

// start triggers the sub-modules of attack network.
func (a *AttackNetworkFactory) start(ctx irrecoverable.SignalerContext) error {
	// creates a connection to all corrupted nodes in the attack network.
	for _, corruptedNodeId := range a.corruptedNodeIds {
		connection, err := a.corruptedConnector.Connect(ctx, corruptedNodeId.NodeID)
		if err != nil {
			return fmt.Errorf("could not establish corruptible connection to node %x: %w", corruptedNodeId.NodeID, err)
		}
		a.corruptedConnections[corruptedNodeId.NodeID] = connection
		a.logger.Info().Hex("node_id", logging.ID(corruptedNodeId.NodeID)).Msg("attacker successfully registered on corrupted node")
	}

	// registers attack network for orchestrator.
	a.orchestrator.Register(a)

	return nil
}

// stop conducts the termination logic of the sub-modules of attack network.
func (a *AttackNetworkFactory) stop() error {
	// tears down connections to corruptible nodes.
	var errors *multierror.Error
	for _, connection := range a.corruptedConnections {
		err := connection.CloseConnection()

		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

// Observe is the inbound message handler of the attack network.
// Instead of dispatching their messages to the networking layer of Flow, the conduits of corrupted nodes
// dispatch the outgoing messages to the attack network by calling the InboundHandler method of it remotely.
func (a *AttackNetwork) Observe(message *insecure.Message) {
	if err := a.processEgressMessage(message); err != nil {
		a.logger.Fatal().Err(err).Msg("could not process message of corrupted node")
	}
}

// processEgressMessage processes incoming messages arrived from corruptible conduits by passing them
// to the orchestrator.
func (a *AttackNetwork) processEgressMessage(message *insecure.Message) error {
	event, err := a.codec.Decode(message.Egress.Payload)
	if err != nil {
		return fmt.Errorf("could not decode observed payload: %w", err)
	}

	sender, err := flow.ByteSliceToId(message.Egress.OriginID)
	if err != nil {
		return fmt.Errorf("could not convert origin id to flow identifier: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(message.Egress.TargetIDs)
	if err != nil {
		return fmt.Errorf("could not convert target ids to flow identifiers: %w", err)
	}

	// making sure events are sent sequentially to orchestrator.
	a.orchestratorMutex.Lock()
	defer a.orchestratorMutex.Unlock()

	err = a.orchestrator.HandleEventFromCorruptedNode(&insecure.EgressEvent{
		OriginId:          sender,
		Channel:           channels.Channel(message.Egress.ChannelID),
		FlowProtocolEvent: event,
		Protocol:          message.Egress.Protocol,
		TargetNum:         message.Egress.TargetNum,
		TargetIds:         targetIds,
	})
	if err != nil {
		return fmt.Errorf("could not handle event by orchestrator: %w", err)
	}

	return nil
}

// SendEgress enforces dissemination of given event via its encapsulated corrupted node networking layer through the Flow network
// An orchestrator decides when to send an egress message on behalf of a corrupted node
func (a *AttackNetworkFactory) SendEgress(event *insecure.EgressEvent) error {
	msg, err := a.eventToEgressMessage(event.OriginId, event.FlowProtocolEvent, event.Channel, event.Protocol, event.TargetNum, event.TargetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = a.sendMessage(msg, event.OriginId)
	if err != nil {
		return fmt.Errorf("could not send egress event from corrupted node: %w", err)
	}

	return nil
}

// SendIngress sends an incoming message from the flow network (from another node that could be or honest or corrupted)
// to the corrupted node. This message was intercepted by the attack network and relayed to the orchestrator before being sent
// to the corrupted node.
func (a *AttackNetworkFactory) SendIngress(event *insecure.IngressEvent) error {
	msg, err := a.eventToIngressMessage(event.OriginID, event.FlowProtocolEvent, event.Channel, event.TargetID)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = a.sendMessage(msg, event.TargetID)
	if err != nil {
		return fmt.Errorf("could not send ingress event to corrupted node: %w", err)
	}
	return nil
}

// sendMessage is a helper function for sending both ingress and egress messages
func (a *AttackNetworkFactory) sendMessage(msg *insecure.Message, lookupId flow.Identifier) error {
	connection, ok := a.corruptedConnections[lookupId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", lookupId)
	}

	err := connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not sent event to corrupted node: %w", err)
	}

	return nil
}

// eventToEgressMessage converts the given application layer event to a protobuf message that is meant to be sent FROM the corrupted node.
func (a *AttackNetworkFactory) eventToEgressMessage(corruptedId flow.Identifier,
	event interface{},
	channel channels.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := a.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	egressMsg := &insecure.EgressMessage{
		ChannelID: channel.String(),
		OriginID:  corruptedId[:],
		TargetNum: num,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}

	return &insecure.Message{
		Egress: egressMsg,
	}, nil
}

// eventToIngressMessage converts the given application layer event to a protobuf message that is meant to be sent TO the corrupted node.
func (a *AttackNetworkFactory) eventToIngressMessage(originId flow.Identifier,
	event interface{},
	channel channels.Channel,
	targetId flow.Identifier) (*insecure.Message, error) {

	payload, err := a.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	ingressMsg := &insecure.IngressMessage{
		ChannelID: channel.String(),
		OriginID:  originId[:], // origin node ID this message was sent from
		TargetID:  targetId[:], // corrupted node ID this message is intended for
		Payload:   payload,
	}

	return &insecure.Message{
		Ingress: ingressMsg,
	}, nil
}
