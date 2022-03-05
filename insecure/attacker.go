package insecure

// AttackerRegister represents the logic of registering an attacker to a corrupted conduit factory so that
// the messages of that factory are relayed to the attacker instead of dispatching to the Flow network.
type AttackerRegister interface {
	// RegisterAttacker registers the attacker by the given network address to all the corrupted conduit factories.
	RegisterAttacker(string) error
}
