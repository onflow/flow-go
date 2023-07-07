package tests

import (
	"github.com/onflow/flow-go/insecure"
)

// mockOrchestrator represents a mock orchestrator that is utilized for composability testing.
type mockOrchestrator struct {
	orchestratorNetwork insecure.OrchestratorNetwork
	// egressEventCorrupter is an injectable function that tampers with the egress events received from the corrupted nodes.
	egressEventCorrupter func(event *insecure.EgressEvent)
	// ingressEventCorrupter is an injectable function that tampers with the ingress events received from the corrupted nodes.
	ingressEventCorrupter func(event *insecure.IngressEvent)
}

var _ insecure.AttackOrchestrator = &mockOrchestrator{}

// HandleEgressEvent implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
//
// In this mock orchestrator, the event corrupter is invoked on the event, and the altered event is sent back to
// the flow network.
func (m *mockOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	m.egressEventCorrupter(event)
	return m.orchestratorNetwork.SendEgress(event)
}

func (m *mockOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	m.ingressEventCorrupter(event)
	return m.orchestratorNetwork.SendIngress(event)
}

func (m *mockOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	m.orchestratorNetwork = orchestratorNetwork
}
