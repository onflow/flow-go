package state

import (
	"strings"

	"github.com/onflow/flow-go/model/flow"
)




// remove reference to flow.RegisterValue and use byte[]
// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Storage interface {
	Set(owner, controller, key string, value flow.RegisterValue) error
	Get(owner, controller, key string) (flow.RegisterValue, error)
	Touch(owner, controller, key string) error
	Delete(owner, controller, key string) error
	RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue)
}

// A BasicStorage is a naive storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type BasicStorage struct {
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

func (m MapLedger) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	data := make(flow.RegisterEntries, 0, len(m.Registers))

	for _, v := range m.Registers {
		data = append(data, v)
	}

	ids := make([]flow.RegisterID, 0, len(m.Registers))
	values := make([]flow.RegisterValue, 0, len(m.Registers))

	for _, v := range data {
		ids = append(ids, v.Key)
		values = append(values, v.Value)
	}

	return ids, values
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
