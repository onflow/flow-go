package wintermute

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
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
func (o *Orchestrator) HandleEventFromCorruptedNode(corruptedId flow.Identifier,
	channel network.Channel,
	event interface{},
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) error {

	corruptedIdentity, ok := o.corruptedIds.ByNodeID(corruptedId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", corruptedId)
	}

	// TODO: how do we keep track of state between calls to HandleEventFromCorruptedNode()?
	// there will be many events sent to the orchestrator and we need a way to co-ordinate all the event calls

	switch corruptedIdentity.Role {
	case flow.RoleExecution:
		// corrupt execution result
		// TODO: do we corrupt a single execution result or all of them?
		// TODO: how do we corrupt each execution result?
		// e.g. honestExecutionResult1.Chunks[0].CollectionIndex = 999
		// TODO: how do we allow unit tests to assert execution result(s) was corrupted? Return type is error

	case flow.RoleVerification:
	default:
		// TODO should we return an error when role is neither EN nor VN?
		panic("unexpected role for Wintermute attack")
	}

	return nil
}
