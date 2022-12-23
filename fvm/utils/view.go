package utils

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// SimpleView provides a simple view for testing and migration purposes.
type SimpleView struct {
	Parent *SimpleView
	Ledger *MapLedger
}

func NewSimpleView() *SimpleView {
	return &SimpleView{
		Ledger: NewMapLedger(),
	}
}

func NewSimpleViewFromPayloads(payloads []ledger.Payload) *SimpleView {
	return &SimpleView{
		Ledger: NewMapLedgerFromPayloads(payloads),
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

	for key, value := range other.Ledger.Registers {
		err := v.Ledger.Set(key.Owner, key.Key, value)
		if err != nil {
			return fmt.Errorf("can not merge: %w", err)
		}
	}

	for k := range other.Ledger.RegisterTouches {
		v.Ledger.RegisterTouches[k] = struct{}{}
	}
	return nil
}

func (v *SimpleView) DropDelta() {
	v.Ledger.Registers = make(map[flow.RegisterID]flow.RegisterValue)
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
		res = append(res, k)
	}
	return res
}

func (v *SimpleView) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	ids := make([]flow.RegisterID, 0, len(v.Ledger.RegisterUpdated))
	values := make([]flow.RegisterValue, 0, len(v.Ledger.RegisterUpdated))
	for key := range v.Ledger.RegisterUpdated {
		ids = append(ids, key)
		values = append(values, v.Ledger.Registers[key])
	}
	return ids, values
}

func (v *SimpleView) Touch(owner, key string) error {
	return v.Ledger.Touch(owner, key)
}

func (v *SimpleView) Delete(owner, key string) error {
	return v.Ledger.Delete(owner, key)
}

func (v *SimpleView) Payloads() []ledger.Payload {
	return v.Ledger.Payloads()
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing and migration purposes.
type MapLedger struct {
	sync.RWMutex
	Registers       map[flow.RegisterID]flow.RegisterValue
	RegisterTouches map[flow.RegisterID]struct{}
	RegisterUpdated map[flow.RegisterID]struct{}
}

// NewMapLedger returns an instance of map ledger (should only be used for
// testing and migration)
func NewMapLedger() *MapLedger {
	return &MapLedger{
		Registers:       make(map[flow.RegisterID]flow.RegisterValue),
		RegisterTouches: make(map[flow.RegisterID]struct{}),
		RegisterUpdated: make(map[flow.RegisterID]struct{}),
	}
}

// NewMapLedger returns an instance of map ledger with entries loaded from
// payloads (should only be used for testing and migration)
func NewMapLedgerFromPayloads(payloads []ledger.Payload) *MapLedger {
	ledger := NewMapLedger()
	for _, entry := range payloads {
		key, err := entry.Key()
		if err != nil {
			panic(err)
		}

		id := flow.RegisterID{
			Owner: string(key.KeyParts[0].Value),
			Key:   string(key.KeyParts[1].Value),
		}

		ledger.Registers[id] = entry.Value()
	}

	return ledger
}

func (m *MapLedger) Set(owner, key string, value flow.RegisterValue) error {
	m.Lock()
	defer m.Unlock()

	k := flow.RegisterID{Owner: owner, Key: key}
	m.RegisterTouches[k] = struct{}{}
	m.RegisterUpdated[k] = struct{}{}
	m.Registers[k] = value
	return nil
}

func (m *MapLedger) Get(owner, key string) (flow.RegisterValue, error) {
	m.Lock()
	defer m.Unlock()

	k := flow.RegisterID{Owner: owner, Key: key}
	m.RegisterTouches[k] = struct{}{}
	return m.Registers[k], nil
}

func (m *MapLedger) Touch(owner, key string) error {
	m.Lock()
	defer m.Unlock()

	m.RegisterTouches[flow.RegisterID{Owner: owner, Key: key}] = struct{}{}
	return nil
}

func (m *MapLedger) Delete(owner, key string) error {
	m.Lock()
	defer m.Unlock()

	delete(m.RegisterTouches, flow.RegisterID{Owner: owner, Key: key})
	return nil
}

func registerIdToLedgerKey(id flow.RegisterID) ledger.Key {
	keyParts := []ledger.KeyPart{
		ledger.NewKeyPart(0, []byte(id.Owner)),
		ledger.NewKeyPart(2, []byte(id.Key)),
	}

	return ledger.NewKey(keyParts)
}

func (m *MapLedger) Payloads() []ledger.Payload {
	m.RLock()
	defer m.RUnlock()

	ret := make([]ledger.Payload, 0, len(m.Registers))
	for id, val := range m.Registers {
		key := registerIdToLedgerKey(id)
		ret = append(ret, *ledger.NewPayload(key, ledger.Value(val)))
	}

	return ret
}
