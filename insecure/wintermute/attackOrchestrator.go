package wintermute

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// Orchestrator encapsulates a stateful implementation of wintermute attack orchestrator logic.
type Orchestrator struct {
	component.Component
	logger       zerolog.Logger
	network      insecure.AttackNetwork
	corruptedIds flow.IdentityList
	allIds       flow.IdentityList // identity of all nodes in the network (including non-corrupted ones)
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(allIds flow.IdentityList, corruptedIds flow.IdentityList, attackNetwork insecure.AttackNetwork, logger zerolog.Logger) *Orchestrator {
	o := &Orchestrator{
		logger:       logger,
		network:      attackNetwork,
		corruptedIds: corruptedIds,
		allIds:       allIds,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			o.start(ctx)

			ready()

			<-ctx.Done()
		}).Build()

	o.Component = cm

	return o
}

// start performs the startup of orchestrator components.
func (o *Orchestrator) start(ctx irrecoverable.SignalerContext) {
	o.network.Start(ctx)
}

// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
func (o *Orchestrator) HandleEventFromCorruptedNode(event *insecure.Event) error {

	// TODO: implement wintermute attack logic.
	panic("implement me")
}
