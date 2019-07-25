package interpreter

import (
	"github.com/raviqqe/hamt"
	"github.com/segmentio/fasthash/fnv1a"
)

/// ActivationKey

type ActivationKey string

func (key ActivationKey) Hash() uint32 {
	return fnv1a.HashString32(string(key))
}

func (key ActivationKey) Equal(other hamt.Entry) bool {
	otherKey, isActivationKey := other.(ActivationKey)
	return isActivationKey && string(otherKey) == string(key)
}

// Activations is a stack of activation records.
// Each entry represents a new scope.
// Variable declarations are performed in the map of the current scope.
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

// Find finds the variable with the given name, in all scopes (current and parent scopes)

func (a *Activations) Find(name string) *Variable {
	current := a.current()
	if current == nil {
		return nil
	}
	value, ok := current.Find(ActivationKey(name)).(*Variable)
	if !ok {
		return nil
	}

	return value
}

func (a *Activations) Set(name string, variable *Variable) {
	current := a.current()
	if current == nil {
		a.PushCurrent()
		current = &a.activations[0]
	}

	count := len(a.activations)
	a.activations[count-1] = current.Insert(ActivationKey(name), variable)
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
