package wintermute

import (
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/adversary"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
)

type Orchestrator struct {
	component.Component
	network      insecure.AttackNetwork
	corruptedIds flow.IdentityList
}

func NewOrchestrator(corruptedIds flow.IdentityList) *Orchestrator {
	return &Orchestrator{
		network:      adversary.NewAttackNetwork(corruptedIds),
		corruptedIds: corruptedIds,
	}
}

func (a *Orchestrator) Handle(msg interface{}) error {
	// TODO: implement wintermute logic
	panic("implement me")
}
