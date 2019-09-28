package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/activations"
	"github.com/raviqqe/hamt"
)

type ValueActivations struct {
	activations *activations.Activations
}

func NewValueActivations() *ValueActivations {
	valueActivations := &activations.Activations{}
	valueActivations.Push(hamt.NewMap())
	return &ValueActivations{
		activations: valueActivations,
	}
}

func (a *ValueActivations) Enter() {
	a.activations.PushCurrent()
}

func (a *ValueActivations) Leave() {
	a.activations.Pop()
}

func (a *ValueActivations) Scoped(f func()) {
	a.Enter()
	defer a.Leave()
	f()
}

func (a *ValueActivations) Set(name string, variable *Variable) {
	a.activations.Set(name, variable)
}

func (a *ValueActivations) Find(name string) *Variable {
	value := a.activations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (a *ValueActivations) Depth() int {
	return a.activations.Depth()
}
