package state

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(owner, controller, key string, value flow.RegisterValue)
	Get(owner, controller, key string) (flow.RegisterValue, error)
	Touch(owner, controller, key string)
	Delete(owner, controller, key string)
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type MapLedger struct {
	Registers       map[string]flow.RegisterValue
	RegisterTouches map[string]bool
}

func NewMapLedger() *MapLedger {
	return &MapLedger{
		Registers:       make(map[string]flow.RegisterValue),
		RegisterTouches: make(map[string]bool),
	}
}

func (m MapLedger) Set(owner, controller, key string, value flow.RegisterValue) {
	k := fullKey(owner, controller, key)
	m.RegisterTouches[k] = true
	m.Registers[k] = value
}

func (m MapLedger) Get(owner, controller, key string) (flow.RegisterValue, error) {

	fmt.Printf("Getting %x %x %s\n", owner, controller, key)

	k := fullKey(owner, controller, key)
	m.RegisterTouches[k] = true
	return m.Registers[k], nil
}

func (m MapLedger) Touch(owner, controller, key string) {
	m.RegisterTouches[fullKey(owner, controller, key)] = true
}

func (m MapLedger) Delete(owner, controller, key string) {
	delete(m.Registers, fullKey(owner, controller, key))
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
