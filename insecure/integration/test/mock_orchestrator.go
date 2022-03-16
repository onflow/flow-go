package test

import (
	"github.com/onflow/flow-go/insecure"
)

type mockOrchestrator struct {
	attackNetwork  insecure.AttackNetwork
	eventCorrupter func(event *insecure.Event)
}

func (m *mockOrchestrator) HandleEventFromCorruptedNode(event *insecure.Event) error {
	m.eventCorrupter(event)
	return m.attackNetwork.Send(event)
}

func (m *mockOrchestrator) WithAttackNetwork(attackNetwork insecure.AttackNetwork) {
	m.attackNetwork = attackNetwork
}
