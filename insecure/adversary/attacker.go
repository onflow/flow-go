package adversary

import (
	"fmt"
	"io"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/component"
)

// Attacker implements the adversarial domain that is orchestrating an attack through corrupted nodes.
type Attacker struct {
	*component.ComponentManager
	logger       zerolog.Logger
	orchestrator insecure.AttackOrchestrator
}

func NewAttacker(address string, orchestrator insecure.AttackOrchestrator) (*Attacker, error) {
	attacker := &Attacker{
		orchestrator: orchestrator,
	}

	s := grpc.NewServer()
	insecure.RegisterAttackerServer(s, attacker)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("could not listen on specified address: %w", err)
	}
	if err = s.Serve(ln); err != nil {
		return nil, fmt.Errorf("could not bind attacker to the tcp listener: %w", err)
	}

}

func (a Attacker) Observe(stream insecure.Attacker_ObserveServer) error {
	for {
		select {
		case <-a.ComponentManager.ShutdownSignal():
			// TODO:
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				a.logger.Info().Msg("attacker closed processing stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err != nil {
				a.logger.Fatal().Err(err).Msg("could not read attacker's stream")
				return stream.SendAndClose(&empty.Empty{})
			}
			if err = a.orchestrator.Handle(msg); err != nil {
				a.logger.Fatal().Err(err).Msg("could not process attacker's message")
				return stream.SendAndClose(&empty.Empty{})
			}
		}
	}
}
