package wintermute

import (
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/adversary"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Orchestrator struct {
	component.Component
	network      insecure.AttackNetwork
	corruptedIds flow.IdentityList
}

func (o Orchestrator) Handle(i interface{}) error {
	panic("implement me")
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(corruptedIds flow.IdentityList) *Orchestrator {
	o := &Orchestrator{
		network:      adversary.NewAttackNetwork(corruptedIds),
		corruptedIds: corruptedIds,
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

func (o *Orchestrator) start(ctx irrecoverable.SignalerContext) {
	o.network.Start(ctx)
}
