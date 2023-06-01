package insecure

// AttackOrchestrator represents the stateful interface that implements a certain type of BFT attack, e.g., wintermute attack.
type AttackOrchestrator interface {
	// HandleEgressEvent implements logic of processing an outgoing flow protocol event from a corrupt node.
	// Corrupt nodes relay all their outgoing events to the orchestrator instead of dispatching them to the network.
	//
	// Note: as a design assumption, this method is invoked sequentially by the OrchestratorNetwork to pass
	// events from corrupt nodes. Hence, no extra concurrency-safe consideration is needed.
	HandleEgressEvent(*EgressEvent) error

	// HandleIngressEvent implements the logic of processing an incoming flow protocol event to a corrupt node.
	// Note: as a design assumption, this method is invoked sequentially by the OrchestratorNetwork to pass
	// events to corrupt nodes. Hence, no extra concurrency-safe consideration is needed.
	HandleIngressEvent(*IngressEvent) error

	// HandleGSEgressEvent implements the logic of processing an outgoing libp2p network event from a corrupt node.
	HandleGSEgressEvent(event *GossipSubEgressEvent) error

	// HandleGSIngressEvent implements the logic of processing an incoming libp2p network event to a corrupt node.
	HandleGSIngressEvent(event *GossipSubIngressEvent) error

	Register(OrchestratorNetwork)
}
