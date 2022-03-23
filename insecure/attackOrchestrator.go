package insecure

// AttackOrchestrator represents the stateful interface that implements a certain type of attack, e.g., wintermute attack.
type AttackOrchestrator interface {
	// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
	//
	// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
	// the attacker instead of dispatching them to the network.
	HandleEventFromCorruptedNode(*Event) error

	WithAttackNetwork(AttackNetwork)
}
