package orchestrator

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
	flownetmsg "github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/logging"
)

// Network implements a middleware for mounting an attack orchestrator and empowering it to communicate with the corrupt nodes.
type Network struct {
	component.Component
	cm                 *component.ComponentManager
	orchestratorMutex  sync.Mutex // to ensure thread-safe calls into orchestrator.
	logger             zerolog.Logger
	orchestrator       insecure.AttackOrchestrator // the mounted orchestrator that implements certain attack logic.
	codec              network.Codec
	corruptNodeIds     flow.IdentityList                                    // identity of the corrupt nodes
	corruptConnections map[flow.Identifier]insecure.CorruptedNodeConnection // existing connections to the corrupt nodes.
	corruptConnector   insecure.CorruptedNodeConnector                      // connection generator to corrupt nodes.
}

var _ insecure.OrchestratorNetwork = &Network{}

func NewOrchestratorNetwork(
	logger zerolog.Logger,
	codec network.Codec,
	orchestrator insecure.AttackOrchestrator,
	connector insecure.CorruptedNodeConnector,
	corruptNodeIds flow.IdentityList) (*Network, error) {

	orchestratorNetwork := &Network{
		orchestrator:       orchestrator,
		logger:             logger,
		codec:              codec,
		corruptConnector:   connector,
		corruptNodeIds:     corruptNodeIds,
		corruptConnections: make(map[flow.Identifier]insecure.CorruptedNodeConnection),
	}

	connector.WithIncomingMessageHandler(orchestratorNetwork.Observe)

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			err := orchestratorNetwork.start(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not start orchestratorNetwork: %w", err))
			}

			ready()

			<-ctx.Done()

			err = orchestratorNetwork.stop()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not stop orchestratorNetwork: %w", err))
			}
		}).Build()

	orchestratorNetwork.Component = cm
	orchestratorNetwork.cm = cm

	return orchestratorNetwork, nil
}

// start triggers the sub-modules of orchestrator network.
func (on *Network) start(ctx irrecoverable.SignalerContext) error {
	// creates a connection to all corrupt nodes in the orchestrator network.
	for _, corruptNodeId := range on.corruptNodeIds {
		connection, err := on.corruptConnector.Connect(ctx, corruptNodeId.NodeID)
		if err != nil {
			return fmt.Errorf("could not establish corrupt connection to node %x: %w", corruptNodeId.NodeID, err)
		}
		on.corruptConnections[corruptNodeId.NodeID] = connection
		on.logger.Info().Hex("node_id", logging.ID(corruptNodeId.NodeID)).Msg("attack orchestrator successfully registered on corrupt node")
	}

	// registers orchestrator network for orchestrator.
	on.orchestrator.Register(on)

	return nil
}

// stop conducts the termination logic of the submodules of orchestrator network.
func (on *Network) stop() error {
	// tears down connections to corrupt nodes.
	var errors *multierror.Error
	for _, connection := range on.corruptConnections {
		err := connection.CloseConnection()

		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

// Observe is the inbound message handler of the orchestrator network.
// Instead of dispatching their messages to the networking layer of Flow, the corrupt nodes
// dispatch both the incoming (i.e., ingress) as well as the outgoing (i.e., egress)
// messages to the orchestrator network by calling the InboundHandler method of it remotely.
func (on *Network) Observe(message *insecure.Message) {
	if message.Ingress == nil && message.Egress == nil {
		// In BFT testing framework, it is a bug to has neither ingress nor egress not set.
		on.logger.Fatal().Msg("could not observe message - both ingress and egress messages can't be nil")
		return // return to avoid changing the behavior by tweaking the log level.
	}
	if message.Ingress != nil && message.Egress != nil {
		// In BFT testing framework, it is a bug to have both ingress and egress messages set.
		on.logger.Fatal().Msg("could not observe message - both ingress and egress messages can't be set")
		return // return to avoid changing the behavior by tweaking the log level.
	}
	if message.Egress != nil {
		if err := on.processEgressMessage(message.Egress); err != nil {
			on.logger.Error().Err(err).Msg("could not process egress message of corrupt node")
			return // return to avoid changing the behavior by tweaking the log level.
		}
	}
	if message.Ingress != nil {
		if err := on.processIngressMessage(message.Ingress); err != nil {
			on.logger.Error().Err(err).Msg("could not process ingress message of corrupt node")
			return // return to avoid changing the behavior by tweaking the log level.
		}
	}
}

// processEgressMessage processes incoming egress messages arrived from corrupt conduits by passing them
// to the orchestrator.
func (on *Network) processEgressMessage(message *insecure.EgressMessage) error {
	event, err := on.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode observed egress payload: %w", err)
	}

	sender, err := flow.ByteSliceToId(message.CorruptOriginID)
	if err != nil {
		return fmt.Errorf("could not convert origin id to flow identifier: %w", err)
	}

	targetIds, err := flow.ByteSlicesToIds(message.TargetIDs)
	if err != nil {
		return fmt.Errorf("could not convert target ids to flow identifiers: %w", err)
	}

	// making sure events are sent sequentially to orchestrator.
	on.orchestratorMutex.Lock()
	defer on.orchestratorMutex.Unlock()

	channel := channels.Channel(message.ChannelID)

	egressEventIDHash, err := flownetmsg.EventId(channel, message.Payload)
	if err != nil {
		return fmt.Errorf("could not create egress event ID: %w", err)
	}

	egressEventID := flow.HashToID(egressEventIDHash)

	err = on.orchestrator.HandleEgressEvent(&insecure.EgressEvent{
		CorruptOriginId:     sender,
		Channel:             channel,
		FlowProtocolEvent:   event,
		FlowProtocolEventID: egressEventID,
		Protocol:            message.Protocol,
		TargetNum:           message.TargetNum,
		TargetIds:           targetIds,
	})
	if err != nil {
		return fmt.Errorf("could not handle egress event by orchestrator: %w", err)
	}

	return nil
}

// processIngressMessage processes incoming ingress messages arrived from corrupt nodes by passing them
// to the orchestrator.
func (on *Network) processIngressMessage(message *insecure.IngressMessage) error {
	event, err := on.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode observed ingress payload: %w", err)
	}

	senderId, err := flow.ByteSliceToId(message.OriginID)
	if err != nil {
		return fmt.Errorf("could not convert origin id to flow identifier: %w", err)
	}

	targetId, err := flow.ByteSliceToId(message.CorruptTargetID)
	if err != nil {
		return fmt.Errorf("could not convert corrupted target id to flow identifier: %w", err)
	}

	// making sure events are sent sequentially to orchestrator.
	on.orchestratorMutex.Lock()
	defer on.orchestratorMutex.Unlock()

	channel := channels.Channel(message.ChannelID)
	ingressEventIDHash, err := flownetmsg.EventId(channel, message.Payload)
	if err != nil {
		return fmt.Errorf("could not create ingress event ID: %w", err)
	}

	ingressEventID := flow.HashToID(ingressEventIDHash)

	err = on.orchestrator.HandleIngressEvent(&insecure.IngressEvent{
		OriginID:            senderId,
		CorruptTargetID:     targetId,
		Channel:             channel,
		FlowProtocolEvent:   event,
		FlowProtocolEventID: ingressEventID,
	})
	if err != nil {
		return fmt.Errorf("could not handle ingress event by orchestrator: %w", err)
	}

	return nil
}

// SendEgress enforces dissemination of given event via its encapsulated corrupt node networking layer through the Flow network.
// An orchestrator decides when to send an egress message on behalf of a corrupt node.
func (on *Network) SendEgress(event *insecure.EgressEvent) error {
	msg, err := on.eventToEgressMessage(event.CorruptOriginId, event.FlowProtocolEvent, event.Channel, event.Protocol, event.TargetNum, event.TargetIds...)
	if err != nil {
		return fmt.Errorf("could not convert egress event to egress message: %w", err)
	}

	err = on.sendMessage(msg, event.CorruptOriginId)
	if err != nil {
		return fmt.Errorf("could not send egress event from corrupt node: %w", err)
	}

	return nil
}

// SendIngress sends an incoming message from the flow network (from another node that could be or honest or corrupt)
// to the corrupt node. This message was intercepted by the orchestrator network and relayed to the orchestrator before being sent
// to the corrupt node.
func (on *Network) SendIngress(event *insecure.IngressEvent) error {
	msg, err := on.eventToIngressMessage(event.OriginID, event.FlowProtocolEvent, event.Channel, event.CorruptTargetID)
	if err != nil {
		return fmt.Errorf("could not convert ingress event to ingress message: %w", err)
	}

	err = on.sendMessage(msg, event.CorruptTargetID)
	if err != nil {
		return fmt.Errorf("could not send ingress event to corrupt node: %w", err)
	}
	return nil
}

// sendMessage is a helper function for sending both ingress and egress messages.
func (on *Network) sendMessage(msg *insecure.Message, corruptNodeId flow.Identifier) error {
	connection, ok := on.corruptConnections[corruptNodeId]
	if !ok {
		return fmt.Errorf("no connection available for corrupt conduit factory to node %x: ", corruptNodeId)
	}

	err := connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not send event to corrupt node: %w", err)
	}

	return nil
}

// eventToEgressMessage converts the given application layer event to a protobuf message that is meant to be sent FROM the corrupt node.
func (on *Network) eventToEgressMessage(corruptId flow.Identifier,
	event interface{},
	channel channels.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := on.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	egressMsg := &insecure.EgressMessage{
		ChannelID:       channel.String(),
		CorruptOriginID: corruptId[:],
		TargetNum:       num,
		TargetIDs:       flow.IdsToBytes(targetIds),
		Payload:         payload,
		Protocol:        protocol,
	}

	return &insecure.Message{
		Egress: egressMsg,
	}, nil
}

// eventToIngressMessage converts the given application layer event to a protobuf message that is meant to be sent TO the corrupt node.
func (on *Network) eventToIngressMessage(originId flow.Identifier,
	event interface{},
	channel channels.Channel,
	targetId flow.Identifier) (*insecure.Message, error) {

	payload, err := on.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	ingressMsg := &insecure.IngressMessage{
		ChannelID:       channel.String(),
		OriginID:        originId[:], // origin node ID this message was sent from
		CorruptTargetID: targetId[:], // corrupt node ID this message is intended for
		Payload:         payload,
	}

	return &insecure.Message{
		Ingress: ingressMsg,
	}, nil
}
