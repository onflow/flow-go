package insecure

type AttackOrchestrator interface {
	Handle(interface{}) error
}
