package utils

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type SimpleView struct {
	Parent *SimpleView
	Ledger *MapLedger
}

func NewSimpleView() *SimpleView {
	return &SimpleView{
		Ledger: &MapLedger{
			Registers:       make(map[string]flow.RegisterEntry),
			RegisterTouches: make(map[string]bool),
		},
	}
}

func (v *SimpleView) NewChild() state.View {
	ch := NewSimpleView()
	ch.Parent = v
	return ch
}

func (v *SimpleView) MergeView(o state.View) error {
	var other *SimpleView
	var ok bool
	if other, ok = o.(*SimpleView); !ok {
		return fmt.Errorf("can not merge: view type mismatch (given: %T, expected:SimpleView)", o)
	}

	for _, item := range other.Ledger.Registers {
		err := v.Ledger.Set(item.Key.Owner, item.Key.Controller, item.Key.Key, item.Value)
		if err != nil {
			return fmt.Errorf("can not merge: %w", err)
		}
	}

	for k := range other.Ledger.RegisterTouches {
		v.Ledger.RegisterTouches[k] = true
	}
	return nil
}

func (v *SimpleView) DropDelta() {
	v.Ledger.Registers = make(map[string]flow.RegisterEntry)
}

func (v *SimpleView) Set(owner, controller, key string, value flow.RegisterValue) error {
	return v.Ledger.Set(owner, controller, key, value)
}

func (v *SimpleView) Get(owner, controller, key string) (flow.RegisterValue, error) {
	value, err := v.Ledger.Get(owner, controller, key)
	if err != nil {
		return nil, err
	}
	if len(value) > 0 {
		return value, nil
	}

	if v.Parent != nil {
		return v.Parent.Get(owner, controller, key)
	}

	return nil, nil
}

func (v *SimpleView) Touch(owner, controller, key string) error {
	return v.Ledger.Touch(owner, controller, key)
}

func (v *SimpleView) Delete(owner, controller, key string) error {
	return v.Ledger.Delete(owner, controller, key)
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type MapLedger struct {
	Registers       map[string]flow.RegisterEntry
	RegisterTouches map[string]bool
}

func NewMapLedger() *MapLedger {
	return &MapLedger{
		Registers:       make(map[string]flow.RegisterEntry),
		RegisterTouches: make(map[string]bool),
	}
}

func (m MapLedger) Set(owner, controller, key string, value flow.RegisterValue) error {
	k := fullKey(owner, controller, key)
	m.RegisterTouches[k] = true
	m.Registers[k] = flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner:      owner,
			Controller: controller,
			Key:        key,
		},
		Value: value,
	}
	return nil
}

func (m MapLedger) Get(owner, controller, key string) (flow.RegisterValue, error) {
	k := fullKey(owner, controller, key)
	m.RegisterTouches[k] = true
	return m.Registers[k].Value, nil
}

func (m MapLedger) Touch(owner, controller, key string) error {
	m.RegisterTouches[fullKey(owner, controller, key)] = true
	return nil
}

func (m MapLedger) Delete(owner, controller, key string) error {
	delete(m.RegisterTouches, fullKey(owner, controller, key))
	return nil
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
