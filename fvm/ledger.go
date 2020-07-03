package fvm

import (
	"crypto/sha256"
	"strings"

	"github.com/dapperlabs/flow-go/model/flow"
)

// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(key flow.RegisterID, value flow.RegisterValue)
	Get(key flow.RegisterID) (flow.RegisterValue, error)
	Touch(key flow.RegisterID)
	Delete(key flow.RegisterID)
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

func (m MapLedger) Set(key flow.RegisterID, value flow.RegisterValue) {
	m.RegisterTouches[string(key)] = true
	m.Registers[string(key)] = value
}

func (m MapLedger) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	m.RegisterTouches[string(key)] = true
	return m.Registers[string(key)], nil
}

func (m MapLedger) Touch(key flow.RegisterID) {
	m.RegisterTouches[string(key)] = true
}

func (m MapLedger) Delete(key flow.RegisterID) {
	delete(m.Registers, string(key))
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}

func fullKeyHash(owner, controller, key string) flow.RegisterID {
	h := sha256.New()
	_, _ = h.Write([]byte(fullKey(owner, controller, key)))
	return h.Sum(nil)
}
