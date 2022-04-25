package test

import (
	"github.com/onflow/flow-go/insecure"
)

// mockOrchestrator represents a mock orchestrator that is utilized for composability testing.
type mockOrchestrator struct {
	attackNetwork insecure.AttackNetwork
	// eventCorrupter is an injectable function that tampers with the given event.
	eventCorrupter func(event *insecure.Event)
}

// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
//
// In this mock orchestrator, the event corrupter is invoked on the event, and the altered event is sent back to
// the flow network.
func (m *mockOrchestrator) HandleEventFromCorruptedNode(event *insecure.Event) error {
	m.eventCorrupter(event)
	return m.attackNetwork.Send(event)
}

func (m *mockOrchestrator) WithAttackNetwork(attackNetwork insecure.AttackNetwork) {
	m.attackNetwork = attackNetwork
}
