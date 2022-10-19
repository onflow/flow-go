package mock

import (
	"github.com/onflow/flow-go/insecure"
)

// Orchestrator represents a mock orchestrator that is utilized for composability testing.
type Orchestrator struct {
	orchestratorNetwork insecure.AttackerNetwork
	// EgressEventCorrupter is an injectable function that tampers with the egress events received from the corrupt nodes.
	EgressEventCorrupter func(event *insecure.EgressEvent)
	// IngressEventCorrupter is an injectable function that tampers with the ingress events received from the corrupt nodes.
	IngressEventCorrupter func(event *insecure.IngressEvent)
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

// HandleEgressEvent implements logic of processing the events received from a corrupt node.
//
// In Corrupt Network Framework for BFT testing, corrupt nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
//
// In this mock orchestrator, the event corrupter is invoked on the event, and the altered event is sent back to
// the flow network.
func (m *Orchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	m.EgressEventCorrupter(event)
	return m.orchestratorNetwork.SendEgress(event)
}

func (m *Orchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	m.IngressEventCorrupter(event)
	return m.orchestratorNetwork.SendIngress(event)
}

func (m *Orchestrator) Register(orchestratorNetwork insecure.AttackerNetwork) {
	m.orchestratorNetwork = orchestratorNetwork
}
