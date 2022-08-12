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

// AttackNetwork implements a middleware for mounting an attack orchestrator and empowering it to communicate with the corrupted nodes.
type AttackNetwork struct {
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
	corruptedNodeIds flow.IdentityList) (*AttackNetwork, error) {

	attackNetwork := &AttackNetwork{
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
func (a *AttackNetwork) start(ctx irrecoverable.SignalerContext) error {
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
	a.orchestrator.WithAttackNetwork(a)

	return nil
}

// stop conducts the termination logic of the sub-modules of attack network.
func (a *AttackNetwork) stop() error {
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

	// making sure events are sequentialized to orchestrator.
	a.orchestratorMutex.Lock()
	defer a.orchestratorMutex.Unlock()

	err = a.orchestrator.HandleEventFromCorruptedNode(&insecure.EgressEvent{
		CorruptedNodeId:   sender,
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

// Send enforces dissemination of given egress event via its encapsulated corrupted node networking layer through the Flow network
func (a *AttackNetwork) Send(event *insecure.EgressEvent) error {

	connection, ok := a.corruptedConnections[event.CorruptedNodeId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", event.CorruptedNodeId)
	}

	msg, err := a.eventToEgressMessage(event.CorruptedNodeId, event.FlowProtocolEvent, event.Channel, event.Protocol, event.TargetNum, event.TargetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not sent event to corrupted node: %w", err)
	}

	return nil
}

// eventToEgressMessage converts the given application layer event to a protobuf message that is meant to be sent to the corrupted node.
func (a *AttackNetwork) eventToEgressMessage(corruptedId flow.Identifier,
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
