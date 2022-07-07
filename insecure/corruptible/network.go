package corruptible

import (
	"context"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/codec"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/utils"
	verutils "github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"io"
	"net"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/logging"
)

// Network is a wrapper around the original flow network, that allows a remote attacker
// to take control over its ingress and egress traffic flows.
type Network struct {
	*component.ComponentManager
	logger                zerolog.Logger
	codec                 flownet.Codec
	mu                    sync.Mutex
	me                    module.Local
	flowNetwork           flownet.Network // original flow network of the node.
	server                *grpc.Server    // touch point of attack network to this factory.
	address               net.Addr
	conduitFactory        *ConduitFactory
	attackerInboundStream insecure.CorruptibleConduitFactory_ConnectAttackerServer // inbound stream to attacker

	receiptHasher  hash.Hasher
	spockHasher    hash.Hasher
	approvalHasher hash.Hasher
}

var _ flownet.Network = &Network{}

func (n *Network) NewCorruptibleNetwork(
	logger zerolog.Logger,
	chainId flow.ChainID,
	address string,
	me module.Local,
	codec flownet.Codec,
	flowNetwork flownet.Network,
	conduitFactory ConduitFactory) (*Network, error) {
	if chainId != flow.BftTestnet {
		panic("illegal chain id for using corruptible network")
	}

	corruptibleNetwork := &Network{
		codec:          codec,
		me:             me,
		logger:         logger.With().Str("component", "corruptible-network").Logger(),
		receiptHasher:  utils.NewExecutionReceiptHasher(),
		spockHasher:    utils.NewSPOCKHasher(),
		approvalHasher: verutils.NewResultApprovalHasher(),
	}

	corruptibleNetwork.conduitFactory = NewCorruptibleConduitFactory(params.Logger, chainId, corruptibleNetwork)

	// instantiating flow network
	if params.Options == nil {
		params.Options = make([]p2p.NetworkOptFunction, 0)
	}
	params.Options = append(params.Options, p2p.WithConduitFactory(corruptibleNetwork.conduitFactory))

	flowNetwork, err := p2p.NewNetwork(params)
	if err != nil {
		return nil, fmt.Errorf("could not initialize Flow network: %w", err)
	}
	corruptibleNetwork.flowNetwork = flowNetwork

	corruptibleNetwork.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			corruptibleNetwork.flowNetwork.Start(ctx)
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			corruptibleNetwork.start(ctx, address)

			ready()

			<-ctx.Done()
			corruptibleNetwork.stop()

		}).Build()

	return corruptibleNetwork, nil
}

// Register serves as the typical network registration of the given message processor on the channel.
// Except, it first wraps the given processor around a corruptible message processor, and then
// registers the corruptible message processor to the original flow network.
func (n *Network) Register(channel flownet.Channel, messageProcessor flownet.MessageProcessor) (flownet.Conduit, error) {
	corruptibleProcessor := NewCorruptibleMessageProcessor(n.logger, messageProcessor)
	// TODO: we can dissolve CCF and instead have a decorator pattern to turn a conduit into
	// a corrupted one?
	conduit, err := n.flowNetwork.Register(channel, corruptibleProcessor)
	if err != nil {
		return nil, fmt.Errorf("could not register corruptible message processor on channel: %s, %w", channel, err)
	}
	return conduit, nil
}

func (n *Network) RegisterBlobService(channel flownet.Channel, store datastore.Batching, opts ...flownet.BlobServiceOption) (flownet.BlobService,
	error) {
	return n.flowNetwork.RegisterBlobService(channel, store, opts...)
}

func (n *Network) RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider flownet.PingInfoProvider) (flownet.PingService, error) {
	return n.flowNetwork.RegisterPingService(pingProtocolID, pingInfoProvider)
}

func (n *Network) ProcessAttackerMessage(stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageServer) error {
	for {
		select {
		case <-n.ComponentManager.ShutdownSignal():
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF || errors.Is(stream.Context().Err(), context.Canceled) {
				n.logger.Info().Msg("attacker closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				n.logger.Fatal().Err(err).Msg("could not read attacker's stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err := n.processAttackerMessage(msg); err != nil {
				n.logger.Fatal().Err(err).Msg("could not process attacker's message")
				return stream.SendAndClose(&empty.Empty{})
			}
		}
	}
}

// processAttackerMessage dispatches the attacker message on the Flow network on behalf of this node.
func (n *Network) processAttackerMessage(msg *insecure.Message) error {
	lg := n.logger.With().
		Str("protocol", insecure.ProtocolStr(msg.Protocol)).
		Uint32("target_num", msg.TargetNum).
		Str("channel", msg.ChannelID).Logger()

	event, err := n.codec.Decode(msg.Payload)
	if err != nil {
		lg.Err(err).Msg("could not decode attacker's message")
		return fmt.Errorf("could not decode message: %w", err)
	}

	lg = n.logger.With().
		Str("flow_protocol_event_type", fmt.Sprintf("%T", event)).Logger()

	switch e := event.(type) {
	case *flow.ExecutionReceipt:
		if len(e.ExecutorSignature) == 0 {
			// empty signature field on execution receipt means attacker is dictating a result to
			// CCF, and the receipt fields must be filled out locally.
			receipt, err := n.generateExecutionReceipt(&e.ExecutionResult)
			if err != nil {
				lg.Err(err).
					Hex("result_id", logging.ID(e.ExecutionResult.ID())).
					Msg("could not generate receipt for attacker's dictated result")
				return fmt.Errorf("could not generate execution receipt for attacker's result: %w", err)
			}
			event = receipt // swaps event with the receipt.
		}

	case *flow.ResultApproval:
		if len(e.VerifierSignature) == 0 {
			// empty signature field on result approval means attacker is dictating an attestation to
			// CCF, and the approval fields must be filled out locally.
			approval, err := n.generateResultApproval(&e.Body.Attestation)
			if err != nil {
				lg.Err(err).
					Hex("result_id", logging.ID(e.Body.ExecutionResultID)).
					Hex("block_id", logging.ID(e.Body.BlockID)).
					Uint64("chunk_index", e.Body.ChunkIndex).
					Msg("could not generate result approval for attacker's dictated attestation")
				return fmt.Errorf("could not generate result approval for attacker's attestation: %w", err)
			}
			event = approval // swaps event with the receipt.
		}
	}

	lg = lg.With().
		Str("event", fmt.Sprintf("%+v", event)).
		Logger()

	targetIds, err := flow.ByteSlicesToIds(msg.TargetIDs)
	if err != nil {
		lg.Err(err).Msg("could not convert target ids from byte to identifiers for attacker's dictated message")
		return fmt.Errorf("could not convert target ids from byte to identifiers: %w", err)
	}

	lg = lg.With().Str("target_ids", fmt.Sprintf("%v", msg.TargetIDs)).Logger()
	err = n.8conduitFactory.sendOnNetwork(event, flownet.Channel(msg.ChannelID), msg.Protocol, uint(msg.TargetNum), targetIds...)
	if err != nil {
		lg.Err(err).Msg("could not send attacker message to the network")
		return fmt.Errorf("could not send attacker message to the network: %w", err)
	}

	lg.Info().Msg("incoming attacker's message dispatched on flow network")

	return nil
}

func (n *Network) start(ctx irrecoverable.SignalerContext, address string) {
	// starts up gRPC server of corruptible network at given address.
	server := grpc.NewServer()
	insecure.RegisterCorruptibleConduitFactoryServer(server, n)
	ln, err := net.Listen(networkingProtocolTCP, address)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
	}
	n.server = server
	n.address = ln.Addr()

	// waits till gRPC server is coming up and running.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		if err = server.Serve(ln); err != nil { // blocking call
			ctx.Throw(fmt.Errorf("could not bind corruptible network to the tcp listener: %w", err))
		}
	}()

	wg.Wait()
}

// stop terminates the corruptible network.
func (n *Network) stop() {
	n.server.Stop()
}

// ServerAddress returns address of the gRPC server that is running by this corruptible network.
func (n *Network) ServerAddress() string {
	return n.address.String()
}

// EngineClosingChannel is called by the conduits of this corruptible network to let it know that the corresponding
// engine of the conduit is not going to use it anymore, so the channel can be closed safely.
func (n *Network) EngineClosingChannel(channel flownet.Channel) error {
	return n.conduitFactory.unregisterChannel(channel)
}

// eventToMessage converts the given application layer event to a protobuf message that is meant to be sent to the attacker.
func (n *Network) eventToMessage(
	event interface{},
	channel flownet.Channel,
	protocol insecure.Protocol,
	targetNum uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	myId := n.me.NodeID()
	return &insecure.Message{
		ChannelID: channel.String(),
		OriginID:  myId[:],
		TargetNum: targetNum,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}, nil
}

func (n *Network) generateExecutionReceipt(result *flow.ExecutionResult) (*flow.ExecutionReceipt, error) {
	// TODO: fill spock secret with dictated spock data from attacker.
	return ingestion.GenerateExecutionReceipt(n.me, n.receiptHasher, n.spockHasher, result, []*delta.SpockSnapshot{})
}

func (n *Network) generateResultApproval(attestation *flow.Attestation) (*flow.ResultApproval, error) {
	// TODO: fill spock secret with dictated spock data from attacker.
	return verifier.GenerateResultApproval(n.me, n.approvalHasher, n.spockHasher, attestation, []byte{})
}

// AttackerRegistered returns whether an attacker has registered on this corruptible network instance.
func (n *Network) AttackerRegistered() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.attackerInboundStream != nil
}

// ConnectAttacker is a gRPC end-point for this corruptible network that lets an attacker register itself to it, so that the attacker can
// control its ingress and egress traffic flow.
// Registering an attacker on a networking layer is an exactly-once immutable operation,
// any second attempt after a successful registration returns an error.
func (n *Network) ConnectAttacker(_ *empty.Empty, stream insecure.CorruptibleConduitFactory_ConnectAttackerServer) error {
	n.mu.Lock()
	n.logger.Info().Msg("attacker registration called arrived")
	if n.attackerInboundStream != nil {
		n.mu.Unlock()
		return fmt.Errorf("could not register a new attacker, one already exists")
	}
	n.attackerInboundStream = stream

	n.mu.Unlock()
	n.logger.Info().Msg("attacker registered successfully")

	// WARNING: this method call should not return through the entire lifetime of this
	// corruptible conduit factory.
	// This is a client streaming gRPC implementation, and the input stream's lifecycle
	// is tightly coupled with the lifecycle of this function call.
	// Once it returns, the client stream is closed forever.
	// Hence, we block the call and wait till a component shutdown.
	<-n.ComponentManager.ShutdownSignal()
	n.logger.Info().Msg("component is shutting down, closing attacker's inbound stream ")

	return nil
}

// HandleOutgoingEvent is called by the conduits generated by this network to relay their outgoing events.
// If there is an attacker connected to this network, the event is dispatched to it.
// Otherwise, the network follows the correct protocol path by sending the message down to the original networking layer
// of Flow to deliver to its targets.
func (n *Network) HandleOutgoingEvent(
	event interface{},
	channel flownet.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) error {

	lg := n.logger.With().
		Hex("corrupted_id", logging.ID(n.me.NodeID())).
		Str("channel", string(channel)).
		Str("protocol", protocol.String()).
		Uint32("target_num", num).
		Str("target_ids", fmt.Sprintf("%v", targetIds)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event)).Logger()

	if !n.AttackerRegistered() {
		// no attacker yet registered, hence sending message on the network following the
		// correct expected behavior.
		lg.Info().Msg("no attacker registered, passing through event")
		return n.conduitFactory.sendOnNetwork(event, channel, protocol, uint(num), targetIds...)
	}

	msg, err := n.eventToMessage(event, channel, protocol, num, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to message: %w", err)
	}

	err = n.attackerInboundStream.Send(msg)
	if err != nil {
		return fmt.Errorf("could not send message to attacker to observe: %w", err)
	}

	lg.Info().Msg("event sent to attacker")
	return nil
}
