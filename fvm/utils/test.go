package utils

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// SimpleView provides a simple view for testing purposes.
type SimpleView struct {
	Parent *SimpleView
	Ledger *MapLedger
}

func NewSimpleView() *SimpleView {
	return &SimpleView{
		Ledger: &MapLedger{
			Registers:       make(map[string]flow.RegisterEntry),
			RegisterTouches: make(map[string]bool),
			RegisterUpdated: make(map[string]bool),
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
		err := v.Ledger.Set(item.Key.Owner, item.Key.Key, item.Value)
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

func (v *SimpleView) Set(owner, key string, value flow.RegisterValue) error {
	return v.Ledger.Set(owner, key, value)
}

func (v *SimpleView) Get(owner, key string) (flow.RegisterValue, error) {
	value, err := v.Ledger.Get(owner, key)
	if err != nil {
		return nil, err
	}
	if len(value) > 0 {
		return value, nil
	}

	if v.Parent != nil {
		return v.Parent.Get(owner, key)
	}

	return nil, nil
}

// returns all the registers that has been touched
func (v *SimpleView) AllRegisters() []flow.RegisterID {
	res := make([]flow.RegisterID, 0, len(v.Ledger.RegisterTouches))
	for k := range v.Ledger.RegisterTouches {
		res = append(res, v.Ledger.Registers[k].Key)
	}
	return res
}

func (v *SimpleView) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	ids := make([]flow.RegisterID, 0, len(v.Ledger.RegisterUpdated))
	values := make([]flow.RegisterValue, 0, len(v.Ledger.RegisterUpdated))
	for k := range v.Ledger.RegisterUpdated {
		ids = append(ids, v.Ledger.Registers[k].Key)
		values = append(values, v.Ledger.Registers[k].Value)
	}
	return ids, values
}

func (v *SimpleView) NumberOfRegistersTouched() int {
	return len(v.Ledger.RegisterTouches)
}

func (v *SimpleView) TotalBytesWrittenToRegisters() int {
	bytesWritten := 0
	for k := range v.Ledger.RegisterUpdated {
		bytesWritten += len(v.Ledger.Registers[k].Value)
	}
	return bytesWritten
}

func (v *SimpleView) Touch(owner, key string) error {
	return v.Ledger.Touch(owner, key)
}

func (v *SimpleView) Delete(owner, key string) error {
	return v.Ledger.Delete(owner, key)
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type MapLedger struct {
	Registers       map[string]flow.RegisterEntry
	RegisterTouches map[string]bool
	RegisterUpdated map[string]bool
}

// NewMapLedger returns an instance of map ledger (should only be used for testing)
func NewMapLedger() *MapLedger {
	return &MapLedger{
		Registers:       make(map[string]flow.RegisterEntry),
		RegisterTouches: make(map[string]bool),
		RegisterUpdated: make(map[string]bool),
	}
}

func (m *MapLedger) Set(owner, key string, value flow.RegisterValue) error {
	k := fullKey(owner, key)
	m.RegisterTouches[k] = true
	m.RegisterUpdated[k] = true
	m.Registers[k] = flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner: owner,
			Key:   key,
		},
		Value: value,
	}
	return nil
}

func (m *MapLedger) Get(owner, key string) (flow.RegisterValue, error) {
	k := fullKey(owner, key)
	m.RegisterTouches[k] = true
	return m.Registers[k].Value, nil
}

func (m *MapLedger) Touch(owner, key string) error {
	m.RegisterTouches[fullKey(owner, key)] = true
	return nil
}

func (m *MapLedger) Delete(owner, key string) error {
	delete(m.RegisterTouches, fullKey(owner, key))
	return nil
}

func fullKey(owner, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, key}, "\x1F")
}
