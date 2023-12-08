package corruptnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/utils"
	verutils "github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/logging"
)

// Network is a wrapper around the original flow network, that allows a remote attack orchestrator
// to take control over its ingress and egress traffic flows.
// A remote attack orchestrator can register itself to this corrupt network.
// Whenever any corrupt conduit receives an event from its engine, it relays the event to this
// network, which in turn is relayed to the register attack orchestrator.
// The attack orchestrator can asynchronously dictate to the network to send messages on behalf of the node.
// Honest message flow:
// Engine -> Conduit -> Flow Networking Layer -> Deliver to targets
//
// Corrupt message flow:
// Engine -> Corrupt Conduit -> Corrupt Network -> Attack Orchestrator (corrupts or passes through) -> Corrupt Network -> Flow Networking Layer
type Network struct {
	*component.ComponentManager
	logger                zerolog.Logger
	codec                 flownet.Codec
	mu                    sync.Mutex
	me                    module.Local
	flowNetwork           flownet.EngineRegistry // original flow network of the node.
	server                *grpc.Server           // touch point of orchestrator network to this factory.
	gRPCListenAddress     net.Addr
	conduitFactory        insecure.CorruptConduitFactory
	attackerInboundStream insecure.CorruptNetwork_ConnectAttackerServer // inbound stream to attack orchestrator

	// We keep the original message processor here so that we can directly send messages to it when
	// attacker dictates to do so.
	// Defined as a concurrent map, as it is accessed concurrently through registration and message processing.
	originalMessageProcessors sync.Map

	receiptHasher  hash.Hasher
	spockHasher    hash.Hasher
	approvalHasher hash.Hasher
}

var _ flownet.EngineRegistry = (*Network)(nil)
var _ insecure.EgressController = (*Network)(nil)
var _ insecure.IngressController = (*Network)(nil)
var _ insecure.CorruptNetworkServer = (*Network)(nil)

func NewCorruptNetwork(
	logger zerolog.Logger,
	chainId flow.ChainID,
	address string,
	me module.Local,
	codec flownet.Codec,
	flowNetwork flownet.EngineRegistry,
	conduitFactory insecure.CorruptConduitFactory) (*Network, error) {
	if chainId != flow.BftTestnet {
		panic("illegal chain id for using corrupt network")
	}

	corruptNetwork := &Network{
		codec:                     codec,
		me:                        me,
		conduitFactory:            conduitFactory,
		flowNetwork:               flowNetwork,
		logger:                    logger.With().Str("component", "corrupt-network").Logger(),
		receiptHasher:             utils.NewExecutionReceiptHasher(),
		spockHasher:               utils.NewSPOCKHasher(),
		approvalHasher:            verutils.NewResultApprovalHasher(),
		originalMessageProcessors: sync.Map{},
	}

	err := corruptNetwork.conduitFactory.RegisterEgressController(corruptNetwork)
	if err != nil {
		return nil, fmt.Errorf("could not register egress controller on conduit factory: %w", err)
	}
	corruptNetwork.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			corruptNetwork.flowNetwork.Start(ctx)
			<-corruptNetwork.flowNetwork.Ready()

			ready()

			<-corruptNetwork.flowNetwork.Done()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			corruptNetwork.start(ctx, address)
			ready()

			<-ctx.Done()
			corruptNetwork.stop()

		}).Build()

	return corruptNetwork, nil
}

// Register serves as the typical network registration of the given message processor on the channel.
// Except, it first wraps the given processor around a corrupt message processor, and then
// registers the corrupt message processor to the original Flow network.
// Returns a non nil error if fails to register the corrupt message processor with the original Flow network.
func (n *Network) Register(channel channels.Channel, messageProcessor flownet.MessageProcessor) (flownet.Conduit, error) {
	corruptProcessor := NewCorruptMessageProcessor(
		n.logger.With().Str("module", "corrupted-message-processor").Hex("corrupt_id", logging.ID(n.me.NodeID())).Logger(),
		messageProcessor,
		n)
	// TODO: we can dissolve CCF and instead have a decorator pattern to turn a conduit into
	// a corrupt one?
	conduit, err := n.flowNetwork.Register(channel, corruptProcessor)
	if err != nil {
		return nil, fmt.Errorf("could not register corrupt message processor on channel: %s, %w", channel, err)
	}
	// we keep the original message processor here so that we can directly send messages to it when
	// attacker dictates to do so.
	n.originalMessageProcessors.Store(channel, messageProcessor)

	return conduit, nil
}

// RegisterBlobService directly invokes the corresponding method on the underlying Flow network instance. It does not perform
// any corruption and passes everything through as it is.
// Returns a non nil error if fails to register with original Flow network.
func (n *Network) RegisterBlobService(channel channels.Channel, store datastore.Batching, opts ...flownet.BlobServiceOption) (flownet.BlobService,
	error) {
	return n.flowNetwork.RegisterBlobService(channel, store, opts...)
}

// RegisterPingService directly invokes the corresponding method on the underlying Flow network instance. It does not perform
// any corruption and passes everything through as it is.
// Returns a non nil error if fails to register with original Flow network.
func (n *Network) RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider flownet.PingInfoProvider) (flownet.PingService, error) {
	return n.flowNetwork.RegisterPingService(pingProtocolID, pingInfoProvider)
}

// ProcessAttackerMessage is the central place for the corrupt network to process messages from an attacker.
// The messages coming from an attacker can be destined to this corrupt node (on behalf of another node) (ingress message) or to another node (on behalf of this corrupt node) (egress message).
// This is a Client Streaming gRPC end-point that allows a registered attack orchestrator to dictate messages to this corrupt
// network.
// The first call to this Client Streaming gRPC method creates the "stream" from attack orchestrator (i.e., client) to this corrupt network
// (i.e., server), where attack orchestrator can send messages through that stream to the corrupt network.
//
// Messages sent from attack orchestrator to this corrupt network are considered dictated in the sense that they are sent on behalf
// of this corrupt network instance on the original Flow network to other Flow nodes.
//
// Returns a fatal error and crashes if message from attacker is invalid (i.e. contains both ingress and egress message or neither ingress nor egress message).
func (n *Network) ProcessAttackerMessage(stream insecure.CorruptNetwork_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-n.ComponentManager.ShutdownSignal():
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF || errors.Is(stream.Context().Err(), context.Canceled) {
				n.logger.Info().Msg("attack orchestrator closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				n.logger.Fatal().Err(err).Msg("could not read attack orchestrator's stream")
				return stream.SendAndClose(&empty.Empty{})
			}

			// this should never happen - one of them (and only one) should be non-nil
			// can't have a message with nil for both ingress and egress
			if msg.Egress == nil && msg.Ingress == nil {
				n.logger.Fatal().Err(err).Msg("could not process attack orchestrator's message - both ingress and egress messages can't be nil")
				return stream.SendAndClose(&empty.Empty{})
			}

			// this should never happen - one of them (and only one) should be not nil
			// can't have a message with not nil for both ingress and egress
			if msg.Egress != nil && msg.Ingress != nil {
				n.logger.Fatal().Err(err).Msg("could not process attack orchestrator's message - both ingress and egress messages can't be set")
				return stream.SendAndClose(&empty.Empty{})
			}

			if msg.Ingress != nil {
				if err := n.processAttackerIngressMessage(msg.Ingress); err != nil {
					n.logger.Fatal().Err(err).Msg("could not process attack orchestrator's ingress message")
					return stream.SendAndClose(&empty.Empty{})
				}
			}

			if msg.Egress != nil {
				if err := n.processAttackerEgressMessage(msg); err != nil {
					n.logger.Fatal().Err(err).Msg("could not process attack orchestrator's egress message")
					return stream.SendAndClose(&empty.Empty{})
				}
			}
		}
	}
}

func (n *Network) processAttackerIngressMessage(msg *insecure.IngressMessage) error {
	lg := n.logger.With().
		Str("channel", msg.ChannelID).Logger()
	event, err := n.codec.Decode(msg.Payload)
	if err != nil {
		lg.Err(err).Msg("could not decode attack orchestrator's ingress message")
		return fmt.Errorf("could not decode ingress message: %w", err)
	}

	senderId, err := flow.ByteSliceToId(msg.OriginID)
	if err != nil {
		return fmt.Errorf("could not convert origin id to flow identifier: %w", err)
	}

	targetId, err := flow.ByteSliceToId(msg.CorruptTargetID)
	if err != nil {
		return fmt.Errorf("could not convert corrupted target id to flow identifier: %w", err)
	}

	lg = n.logger.With().
		Hex("sender_id", logging.ID(senderId)).
		Hex("corrupted_target_id", logging.ID(targetId)).
		Str("flow_protocol_event_type", fmt.Sprintf("%T", event)).Logger()

	if targetId != n.me.NodeID() {
		lg.Fatal().Msg("corrupt network received ingress message for a different node")
	}

	lg.Info().Msg("corrupt network received ingress message")

	// As this ingress message is dictated by the attack orchestrator, we need to send it to the original message processor
	// instead of the corrupt one. The reason is the corrupt one always routes the ingress messages back to the attack
	// orchestrator, which yields an infinite loop.
	originalProcessor, ok := n.originalMessageProcessors.Load(channels.Channel(msg.ChannelID))
	if !ok {
		lg.Fatal().Msg("corrupt network received ingress message for an unknown channel")
	}

	err = originalProcessor.(flownet.MessageProcessor).Process(channels.Channel(msg.ChannelID), senderId, event)
	if err != nil {
		lg.Fatal().Err(err).Msg("could not relay ingress message to original processor")
	}

	lg.Info().Msg("corrupt network relayed ingress message to original processor")

	return nil
}

// processAttackerEgressMessage dispatches the attack orchestrator message on the Flow network on behalf of this node.
func (n *Network) processAttackerEgressMessage(msg *insecure.Message) error {
	lg := n.logger.With().
		Str("protocol", insecure.ProtocolStr(msg.Egress.Protocol)).
		Uint32("target_num", msg.Egress.TargetNum).
		Str("channel", msg.Egress.ChannelID).Logger()

	event, err := n.codec.Decode(msg.Egress.Payload)
	if err != nil {
		lg.Err(err).Msg("could not decode attack orchestrator's egress message")
		return fmt.Errorf("could not decode egress message: %w", err)
	}

	lg = n.logger.With().
		Str("flow_protocol_event_type", fmt.Sprintf("%T", event)).Logger()

	switch e := event.(type) {
	case *flow.ExecutionReceipt:
		if len(e.ExecutorSignature) == 0 {
			// empty signature field on execution receipt means attack orchestrator is dictating a result to
			// CCF, and the receipt fields must be filled out locally.
			receipt, err := n.generateExecutionReceipt(&e.ExecutionResult)
			if err != nil {
				lg.Err(err).
					Hex("result_id", logging.ID(e.ExecutionResult.ID())).
					Msg("could not generate receipt for attack orchestrator's dictated result")
				return fmt.Errorf("could not generate execution receipt for attack orchestrator's result: %w", err)
			}
			event = receipt // swaps event with the receipt.
		}

	case *flow.ResultApproval:
		if len(e.VerifierSignature) == 0 {
			// empty signature field on result approval means attack orchestrator is dictating an attestation to
			// CCF, and the approval fields must be filled out locally.
			approval, err := n.generateResultApproval(&e.Body.Attestation)
			if err != nil {
				lg.Err(err).
					Hex("result_id", logging.ID(e.Body.ExecutionResultID)).
					Hex("block_id", logging.ID(e.Body.BlockID)).
					Uint64("chunk_index", e.Body.ChunkIndex).
					Msg("could not generate result approval for attack orchestrator's dictated attestation")
				return fmt.Errorf("could not generate result approval for attack orchestrator's attestation: %w", err)
			}
			event = approval // swaps event with the receipt.
		}
	}

	lg = lg.With().
		Str("event", fmt.Sprintf("%+v", event)).
		Logger()

	targetIds, err := flow.ByteSlicesToIds(msg.Egress.TargetIDs)
	if err != nil {
		lg.Err(err).Msg("could not convert target ids from byte to identifiers for attack orchestrator's dictated egress message")
		return fmt.Errorf("could not convert target ids from byte to identifiers: %w", err)
	}

	lg = lg.With().Str("target_ids", fmt.Sprintf("%v", msg.Egress.TargetIDs)).Logger()
	err = n.conduitFactory.SendOnFlowNetwork(event, channels.Channel(msg.Egress.ChannelID), msg.Egress.Protocol, uint(msg.Egress.TargetNum), targetIds...)
	if err != nil {
		lg.Err(err).Msg("could not send attack orchestrator egress message to the network")
		return fmt.Errorf("could not send attack orchestrator egress message to the network: %w", err)
	}

	lg.Info().Msg("incoming attack orchestrator's message dispatched on flow network")

	return nil
}

func (n *Network) start(ctx irrecoverable.SignalerContext, gRPCListenAddress string) {
	// starts up gRPC server of corrupt network at given address.
	server := grpc.NewServer()
	insecure.RegisterCorruptNetworkServer(server, n)
	ln, err := net.Listen(networkingProtocolTCP, gRPCListenAddress)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
	}
	n.server = server
	n.gRPCListenAddress = ln.Addr()

	// waits till gRPC server is coming up and running.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		if err = server.Serve(ln); err != nil { // blocking call
			ctx.Throw(fmt.Errorf("could not bind corrupt network to the tcp listener: %w", err))
		}
	}()

	wg.Wait()
}

// stop terminates the corrupt network.
func (n *Network) stop() {
	n.server.Stop()
}

// ServerAddress returns listen address of the gRPC server that is running by this corrupt network.
func (n *Network) ServerAddress() string {
	return n.gRPCListenAddress.String()
}

// EngineClosingChannel is called by the conduits of this corrupt network to let it know that the corresponding
// engine of the conduit is not going to use it anymore, so the channel can be closed safely.
func (n *Network) EngineClosingChannel(channel channels.Channel) error {
	return n.conduitFactory.UnregisterChannel(channel)
}

// eventToEgressMessage converts the given application layer event to a protobuf message that is meant to be sent to the attack orchestrator.
func (n *Network) eventToEgressMessage(
	event interface{},
	channel channels.Channel,
	protocol insecure.Protocol,
	targetNum uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	myId := n.me.NodeID()

	egressMsg := &insecure.EgressMessage{
		ChannelID:       channel.String(),
		CorruptOriginID: myId[:],
		TargetNum:       targetNum,
		TargetIDs:       flow.IdsToBytes(targetIds),
		Payload:         payload,
		Protocol:        protocol,
	}

	msg := &insecure.Message{
		Egress: egressMsg,
	}

	return msg, nil
}

func (n *Network) eventToIngressMessage(event interface{}, channel channels.Channel, originId flow.Identifier) (*insecure.Message, error) {
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}
	myId := n.me.NodeID()

	ingressMsg := &insecure.IngressMessage{
		ChannelID:       channel.String(),
		OriginID:        originId[:],
		CorruptTargetID: myId[:],
		Payload:         payload,
	}

	msg := &insecure.Message{
		Ingress: ingressMsg,
	}

	return msg, nil
}

func (n *Network) generateExecutionReceipt(result *flow.ExecutionResult) (*flow.ExecutionReceipt, error) {
	// TODO: fill spock secret with dictated spock data from attack orchestrator.
	return computer.GenerateExecutionReceipt(n.me, n.receiptHasher, result, []crypto.Signature{})
}

func (n *Network) generateResultApproval(attestation *flow.Attestation) (*flow.ResultApproval, error) {
	// TODO: fill spock secret with dictated spock data from attack orchestrator.
	return verifier.GenerateResultApproval(n.me, n.approvalHasher, n.spockHasher, attestation, []byte{})
}

// AttackerRegistered returns whether an attack orchestrator has registered on this corrupt network instance.
func (n *Network) AttackerRegistered() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.attackerInboundStream != nil
}

// ConnectAttacker is a blocking Server Streaming gRPC end-point for this corrupt network that registers an attacker to the corrupt network,
// so that the attacker can control its ingress and egress traffic flow.
//
// An attack orchestrator (i.e., client) remote call to this function will return immediately on the attack orchestrator's side. However,
// here on the server (i.e., corrupt network) side, the call remains blocking through the lifecycle of the server.
// The reason is the local gRPC stub on this corrupt network (i.e., server) acts as a broker between client call to
// this server method. The broker returns the call on the client side immediately by creating the stream from server to
// the client, i.e., server streaming.
// However, that stream is only alive through the lifecycle of the server. So, this method should only return when the server
// is really shut down, hence closing the stream on the client side, as client should expect no more messages streamed from
// server.
//
// Registering an attack orchestrator on a networking layer is an exactly-once immutable operation,
// any second attempt after a successful registration returns an error.
func (n *Network) ConnectAttacker(_ *empty.Empty, stream insecure.CorruptNetwork_ConnectAttackerServer) error {
	n.mu.Lock()
	n.logger.Info().Msg("attack orchestrator registration called arrived")
	if n.attackerInboundStream != nil {
		n.mu.Unlock()
		return fmt.Errorf("could not register a new attack orchestrator, one already exists")
	}
	n.attackerInboundStream = stream

	n.mu.Unlock()
	n.logger.Info().Msg("attack orchestrator registered successfully")

	// WARNING: this method call should not return through the entire lifetime of this
	// corrupt conduit factory.
	// This is a client streaming gRPC implementation, and the input stream's lifecycle
	// is tightly coupled with the lifecycle of this function call.
	// Once it returns, the client stream is closed forever.
	// Hence, we block the call and wait till a component shutdown.
	<-n.ComponentManager.ShutdownSignal()
	n.logger.Info().Msg("component is shutting down, closing attack orchestrator's inbound stream ")

	return nil
}

// HandleOutgoingEvent is called by the conduits generated by this network to relay their outgoing events.
// If there is an attack orchestrator connected to this network, the event is dispatched to it.
// Otherwise, the network follows the correct protocol path by sending the message down to the original networking layer
// of Flow to deliver to its targets.
func (n *Network) HandleOutgoingEvent(
	event interface{},
	channel channels.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) error {

	lg := n.logger.With().
		Hex("corrupt_id", logging.ID(n.me.NodeID())).
		Str("channel", string(channel)).
		Str("protocol", protocol.String()).
		Uint32("target_num", num).
		Str("target_ids", fmt.Sprintf("%v", targetIds)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event)).Logger()

	if !n.AttackerRegistered() {
		// no attack orchestrator yet registered, hence sending message on the network following the
		// correct expected behavior.
		lg.Info().Msg("no attack orchestrator registered, passing through event")
		return n.conduitFactory.SendOnFlowNetwork(event, channel, protocol, uint(num), targetIds...)
	}

	msg, err := n.eventToEgressMessage(event, channel, protocol, num, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = n.attackerInboundStream.Send(msg)
	if err != nil {
		return fmt.Errorf("could not send message to attack orchestrator to observe: %w", err)
	}

	lg.Info().Msg("event sent to attack orchestrator")
	return nil
}

// HandleIncomingEvent is called on the incoming messages to this corrupt node.
// Returns true if an attacker is registered and false otherwise.
// Honest node (i.e., not running with a corrupt network) message flow: Flow Networking Layer -> Honest Engine
// Corrupt node (i.e., running with a corrupt network) message flow (with attacker registered):
// Flow Networking Layer -> Corrupt Network -> Attack Orchestrator (mute or passthrough) -> Corrupt Network -> Honest / Corrupt Engine
func (n *Network) HandleIncomingEvent(event interface{}, channel channels.Channel, originId flow.Identifier) bool {
	lg := n.logger.With().
		Hex("corrupt_id", logging.ID(n.me.NodeID())).
		Str("channel", string(channel)).
		Str("origin_id", fmt.Sprintf("%v", originId)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event)).Logger()

	if !n.AttackerRegistered() {
		// no attack orchestrator registered, so return to message processor to pass back to flow network
		lg.Info().Msg("no attack orchestrator registered, passing through event")
		return false
	}

	msg, err := n.eventToIngressMessage(event, channel, originId)
	if err != nil {
		lg.Error().Err(err).Msg("could not convert event to ingress message")
		return false
	}

	err = n.attackerInboundStream.Send(msg)
	if err != nil {
		lg.Error().Err(err).Msg("could not send message to attack orchestrator to observe")
		return false
	}

	lg.Info().Msg("ingress event successfully sent to attack orchestrator")
	return true
}
