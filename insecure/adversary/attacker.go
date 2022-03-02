package adversary

import (
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

// Attacker implements the adversarial domain that is orchestrating an attack through corrupted nodes.
type Attacker struct {
	corruptedIds flow.IdentityList
	network      insecure.AttackNetwork
	orchestrator insecure.AttackOrchestrator
}

func NewAttacker(address string, corruptedIds flow.IdentityList, orchestrator insecure.AttackOrchestrator) (*Attacker, error) {
	attacker := &Attacker{
		corruptedIds: corruptedIds,
		orchestrator: orchestrator,
	}

	s := grpc.NewServer()
	insecure.RegisterAttackerServer(s, attacker)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("could not listen on specified address: %w", err)
	}
	if err := s.Serve(ln); err != nil {
		return nil, fmt.Errorf("could not bind attacker to the tcp listener: %w", err)
	}

}

func (a Attacker) Start() {
	a.orchestrator.Start()
}

func (a Attacker) Observe(server insecure.Attacker_ObserveServer) error {
	panic("implement me")
}
