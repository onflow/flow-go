package insecure

import (
	"github.com/onflow/flow-go/module/component"
)

type AttackOrchestrator interface {
	component.Component
	Handle(interface{}) error
}
