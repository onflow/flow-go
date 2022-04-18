package wintermute

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

// Orchestrator encapsulates a stateful implementation of wintermute attack orchestrator logic.
type Orchestrator struct {
	logger       zerolog.Logger
	network      insecure.AttackNetwork
	corruptedIds flow.IdentityList
	allIds       flow.IdentityList // identity of all nodes in the network (including non-corrupted ones)
}

func NewOrchestrator(allIds flow.IdentityList, corruptedIds flow.IdentityList, logger zerolog.Logger) *Orchestrator {
	o := &Orchestrator{
		logger:       logger,
		corruptedIds: corruptedIds,
		allIds:       allIds,
	}

	return o
}

func (o *Orchestrator) WithAttackNetwork(network insecure.AttackNetwork) {
	o.network = network
}

// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
func (o *Orchestrator) HandleEventFromCorruptedNode(event *insecure.Event) error {

	// TODO: implement wintermute attack logic.
	panic("implement me")
}
