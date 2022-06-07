package attacknetwork

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
	"github.com/onflow/flow-go/utils/logging"
)

type runtime string

const (
	networkingProtocolTCP = "tcp"
	//dockerLocalHost       = "host.docker.internal"
	dockerRuntime    = runtime("docker")
	localhostRuntime = runtime("localhost")
)

// AttackNetwork implements a middleware for mounting an attack orchestrator and empowering it to communicate with the corrupted nodes.
type AttackNetwork struct {
	component.Component
	rt                   runtime // denotes whether corrupted nodes are running in docker or localhost
	cm                   *component.ComponentManager
	logger               zerolog.Logger
	address              string                      // address on which the orchestrator is reachable from corrupted nodes.
	server               *grpc.Server                // touch point of corrupted nodes with the mounted orchestrator.
	orchestrator         insecure.AttackOrchestrator // the mounted orchestrator that implements certain attack logic.
	codec                network.Codec
	corruptedNodeIds     flow.IdentityList                                    // identity of the corrupted nodes
	corruptedConnections map[flow.Identifier]insecure.CorruptedNodeConnection // existing connections to the corrupted nodes.
	corruptedConnector   insecure.CorruptedNodeConnector                      // connection generator to corrupted nodes.

}

// WithLocalHostRuntime denotes the attack network that corrupted nodes are running on the localhost.
// This is typically the case for unit testing the attack network.
func WithLocalHostRuntime(a *AttackNetwork) {
	a.rt = localhostRuntime
}

func NewAttackNetwork(
	logger zerolog.Logger,
	address string,
	codec network.Codec,
	orchestrator insecure.AttackOrchestrator,
	connector insecure.CorruptedNodeConnector,
	corruptedNodeIds flow.IdentityList, opts ...func(*AttackNetwork)) (*AttackNetwork, error) {

	attackNetwork := &AttackNetwork{
		rt:                   dockerRuntime,
		orchestrator:         orchestrator,
		logger:               logger,
		codec:                codec,
		corruptedConnector:   connector,
		corruptedNodeIds:     corruptedNodeIds,
		corruptedConnections: make(map[flow.Identifier]insecure.CorruptedNodeConnection),
	}

	for _, opt := range opts {
		opt(attackNetwork)
	}

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			err := attackNetwork.start(ctx, address)
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
func (a *AttackNetwork) start(ctx irrecoverable.SignalerContext, address string) error {
	// starts up gRPC server of attack network at given address.
	s := grpc.NewServer()
	insecure.RegisterAttackerServer(s, a)
	fmt.Printf("attackNetwork>start>about to net.Listen on address: %s\n", address)
	ln, err := net.Listen(networkingProtocolTCP, address)
	fmt.Printf("attackNetwork>start>called net.Listen()\n")
	if err != nil {
		fmt.Printf("attackNetwork>start>err != nil")
		ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
	}
	a.server = s
	a.address = ln.Addr().String()
	fmt.Printf("attackNetwork>start>a.address (before dockerRuntime check)=%s\n", a.address)
	if a.rt == dockerRuntime {
		fmt.Printf("attackNetwork>start>a.rt == dockerRuntime\n")
		// since corrupted nodes are running on docker, attacker registers itself
		// with a docker local host address, hence being reachable from internal docker
		// network.
		_, port, err := net.SplitHostPort(a.address)
		fmt.Printf("attackNetwork>start>port=%s\n", port)
		if err != nil {
			return fmt.Errorf("could not split host and port for address: %s", a.address)
		}
		//a.address = fmt.Sprintf("%s:%s", dockerLocalHost, port)
		fmt.Printf("attackNetwork>start>a.address (within dockerRuntime check)=%s\n", a.address)
	}

	fmt.Printf("attackNetwork>start>about to a.corruptedConnector.WithAttackerAddress(%s)\n", a.address)
	a.corruptedConnector.WithAttackerAddress(a.address)

	fmt.Printf("attackNetwork>start>about to create wait group\n")
	wg := sync.WaitGroup{}
	fmt.Printf("attackNetwork>start>about to wg.Add(1)\n")
	wg.Add(1)
	fmt.Printf("attackNetwork>start>about to call goroutine\n")
	go func() {
		fmt.Printf("attackNetwork>start>go func()>about to wg.Done()\n")
		wg.Done()
		fmt.Printf("attackNetwork>start>go func()>about to s.Serve(ln) - blocking call\n")
		if err = s.Serve(ln); err != nil { // blocking call
			ctx.Throw(fmt.Errorf("could not bind attackNetwork to the tcp listener: %w", err))
		}
		fmt.Printf("attackNetwork>start>go func()>about return from s.Server(ln)\n")
	}()

	fmt.Printf("attackNetwork>start>about to wg.Wait()\n")
	// waits till gRPC server starts serving.
	wg.Wait()

	fmt.Printf("attackNetwork>start>finished wg.Wait()\n")
	// creates a connection to all corrupted nodes in the attack network.
	for _, corruptedNodeId := range a.corruptedNodeIds {
		fmt.Printf("attackNetwork>start>connecting to corrupted node ID %s\n", corruptedNodeId.NodeID.String())
		connection, err := a.corruptedConnector.Connect(ctx, corruptedNodeId.NodeID)
		if err != nil {
			return fmt.Errorf("could not establish corruptible connection to node %x: %w", corruptedNodeId.NodeID, err)
		}
		fmt.Printf("attackNetwork>start>adding connection to map; corruptedNodeId.NodeID=%s\n", corruptedNodeId.NodeID.String())
		a.corruptedConnections[corruptedNodeId.NodeID] = connection
		a.logger.Info().Hex("node_id", logging.ID(corruptedNodeId.NodeID)).Msg("attacker successfully registered on corrupted node")
	}

	fmt.Printf("attackNetwork>start>about to a.orchestrator.WithAttackNetwork(a)\n")
	// registers attack network for orchestrator.
	a.orchestrator.WithAttackNetwork(a)
	fmt.Printf("attackNetwork>start>just did to a.orchestrator.WithAttackNetwork(a)\n")
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

	// tears down the attack server itself.
	a.server.Stop()

	return errors.ErrorOrNil()
}

// ServerAddress returns the address on which the orchestrator is reachable from corrupted nodes.
func (a AttackNetwork) ServerAddress() string {
	return a.address
}

// Observe implements the gRPC interface of the attack network that is exposed to the corrupted conduits.
// Instead of dispatching their messages to the networking layer of Flow, the conduits of corrupted nodes
// dispatch the outgoing messages to the attack network by calling the Observe method of it remotely.
func (a *AttackNetwork) Observe(stream insecure.Attacker_ObserveServer) error {
	for {
		select {
		case <-a.cm.ShutdownSignal():
			// attack network terminated, hence terminating this loop.
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				a.logger.Info().Msg("attack network closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				return fmt.Errorf("could not read corrupted node's stream: %w", err)
			}

			if err = a.processMessageFromCorruptedNode(msg); err != nil {
				a.logger.Fatal().Err(err).Msg("could not process message of corrupted node")
				return stream.SendAndClose(&empty.Empty{})
			}
		}
	}
}

// processMessageFromCorruptedNode processes incoming messages arrived from corruptible conduits by passing them
// to the orchestrator.
func (a *AttackNetwork) processMessageFromCorruptedNode(message *insecure.Message) error {
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
		CorruptedNodeId:   sender,
		Channel:           network.Channel(message.ChannelID),
		FlowProtocolEvent: event,
		Protocol:          message.Protocol,
		TargetNum:         message.TargetNum,
		TargetIds:         targetIds,
	})
	if err != nil {
		return fmt.Errorf("could not handle event by orchestrator: %w", err)
	}

	return nil
}

// Send enforces dissemination of given event via its encapsulated corrupted node networking layer through the Flow network
func (a *AttackNetwork) Send(event *insecure.Event) error {

	connection, ok := a.corruptedConnections[event.CorruptedNodeId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", event.CorruptedNodeId)
	}

	msg, err := a.eventToMessage(event.CorruptedNodeId, event.FlowProtocolEvent, event.Channel, event.Protocol, event.TargetNum, event.TargetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = connection.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("could not sent event to corrupted node: %w", err)
	}

	return nil
}

// eventToMessage converts the given application layer event to a protobuf message that is meant to be sent to the corrupted node.
func (a *AttackNetwork) eventToMessage(corruptedId flow.Identifier,
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
		TargetNum: num,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}, nil
}
