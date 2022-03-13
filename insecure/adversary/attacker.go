package adversary

import (
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

const networkingProtocolTCP = "tcp"

// Attacker implements the adversarial domain that is orchestrating an attack through corrupted nodes.
type Attacker struct {
	component.Component
	address      net.Addr
	server       *grpc.Server
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
			// starts up gRPC server of attacker at given address.
			s := grpc.NewServer()
			insecure.RegisterAttackerServer(s, attacker)
			ln, err := net.Listen(networkingProtocolTCP, address)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not listen on specified address: %w", err))
			}
			attacker.server = s
			attacker.address = ln.Addr()

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
			attacker.start(ctx)

			ready()

			<-ctx.Done()
			attacker.Stop()
		}).Build()

	attacker.Component = cm
	attacker.cm = cm

	return attacker, nil
}

// start triggers the sub-modules of attacker.
func (a *Attacker) start(ctx irrecoverable.SignalerContext) {
	a.orchestrator.Start(ctx)
}

// Stop stops the sub-modules of attacker.
func (a *Attacker) Stop() {
	a.server.Stop()
}

func (a Attacker) Address() net.Addr {
	return a.address
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
