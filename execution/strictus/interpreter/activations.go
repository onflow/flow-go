package interpreter

import (
	"bamboo-emulator/execution/strictus/ast"
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

// Activations is a stack of activation records (identifier bindings)
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

func (a *Activations) Find(name string) ast.Repr {
	current := a.current()
	if current == nil {
		return nil
	}
	value, ok := current.Find(ActivationKey(name)).(ast.Repr)
	if !ok {
		return nil
	}

	return value
}

func (a *Activations) Set(name string, value ast.Repr) {
	current := a.current()
	if current == nil {
		a.Push()
		current = &a.activations[0]
	}

	count := len(a.activations)
	a.activations[count-1] = current.Insert(ActivationKey(name), value)
}

func (a *Activations) Push() {
	current := a.current()
	if current == nil {
		first := hamt.NewMap()
		current = &first
	}
	a.activations = append(
		a.activations,
		*current,
	)
}

func (a *Activations) Pop() {
	count := len(a.activations)
	if count < 1 {
		return
	}
	a.activations = a.activations[:count-1]
}
