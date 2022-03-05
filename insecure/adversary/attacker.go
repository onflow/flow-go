package adversary

import (
	"fmt"
	"io"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

// Attacker implements the adversarial domain that is orchestrating an attack through corrupted nodes.
type Attacker struct {
	component.Component
	logger       zerolog.Logger
	orchestrator insecure.AttackOrchestrator
	codec        network.Codec
	cm           *component.ComponentManager
}

func NewAttacker(logger zerolog.Logger, address string, codec network.Codec, orchestrator insecure.AttackOrchestrator) (*Attacker, error) {
	attacker := &Attacker{
		orchestrator: orchestrator,
		logger:       logger,
		codec:        codec,
	}

	// setting lifecycle management module.
	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			attacker.start(ctx)

			ready()

			<-ctx.Done()
		}).Build()

	attacker.Component = cm
	attacker.cm = cm

	if err := attacker.listenAndServe(address); err != nil {
		return nil, fmt.Errorf("could not start up a gRPC server for attacker: %w", err)
	}

	return attacker, nil
}

// start triggers the sub-modules of attacker.
func (a *Attacker) start(ctx irrecoverable.SignalerContext) {
	a.orchestrator.Start(ctx)
}

// listenAndServe establishes an attacker gRPC server on the specified address.
func (a *Attacker) listenAndServe(address string) error {
	s := grpc.NewServer()
	insecure.RegisterAttackerServer(s, a)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("could not listen on specified address: %w", err)
	}
	if err = s.Serve(ln); err != nil {
		return fmt.Errorf("could not bind attacker to the tcp listener: %w", err)
	}

	return nil
}

// Observe implements the gRPC interface of attacker that is exposed to the corrupted conduits.
// Instead of dispatching their messages to the networking layer of Flow, the conduits of corrupted nodes
// dispatch the outgoing messages to the attacker by calling the Observe method of it remotely.
func (a *Attacker) Observe(stream insecure.Attacker_ObserveServer) error {
	for {
		select {
		case <-a.cm.ShutdownSignal():
			// TODO:
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				a.logger.Info().Msg("attacker closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				a.logger.Fatal().Err(err).Msg("could not read stream of corrupted node")
				return stream.SendAndClose(&empty.Empty{})
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

	channel := network.Channel(message.ChannelID)
	if err = a.orchestrator.HandleEventFromCorruptedNode(sender, channel, event, message.Protocol, message.Targets, targetIds...); err != nil {
		return fmt.Errorf("could not handle event by orchestrator: %w", err)
	}

	return nil
}
