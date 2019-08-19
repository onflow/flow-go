package activations

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
)

// Activations is a stack of activation records.
// Each entry represents a new scope.
//
type Activations struct {
	activations []hamt.Map
}

func (a *Activations) current() *hamt.Map {
	count := len(a.activations)
	if count < 1 {
		return nil
	}
	current := a.activations[count-1]
	return &current
}

func (a *Activations) Find(key string) interface{} {
	current := a.current()
	if current == nil {
		return nil
	}
	return current.Find(common.StringKey(key))
}

func (a *Activations) Set(name string, value interface{}) {
	current := a.current()
	if current == nil {
		a.PushCurrent()
		current = &a.activations[0]
	}

	count := len(a.activations)
	a.activations[count-1] = current.
		Insert(common.StringKey(name), value)
}

func (a *Activations) PushCurrent() {
	current := a.current()
	if current == nil {
		first := hamt.NewMap()
		current = &first
	}
	a.Push(*current)
}

func (a *Activations) Push(activation hamt.Map) {
	a.activations = append(
		a.activations,
		activation,
	)
}

func (a *Activations) Pop() {
	count := len(a.activations)
	if count < 1 {
		return
	}
	a.activations = a.activations[:count-1]
}

func (a *Activations) CurrentOrNew() hamt.Map {
	current := a.current()
	if current == nil {
		return hamt.NewMap()
	}

	return *current
}

func (a *Activations) Depth() int {
	return len(a.activations)
}
