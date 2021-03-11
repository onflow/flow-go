package state

import (
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	NewChild() View
	MergeView(child View)
	Ledger
}

// Rename this to Storage
// remove reference to flow.RegisterValue and use byte[]
// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(owner, controller, key string, value flow.RegisterValue) error
	Get(owner, controller, key string) (flow.RegisterValue, error)
	Touch(owner, controller, key string) error
	Delete(owner, controller, key string) error
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
