package state

import (
	"strings"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
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
	k := RegisterID(owner, controller, key)
	m.RegisterTouches[string(k)] = true
	m.Registers[string(k)] = value
}

func (m MapLedger) Get(owner, controller, key string) (flow.RegisterValue, error) {
	k := RegisterID(owner, controller, key)
	m.RegisterTouches[string(k)] = true
	return m.Registers[string(k)], nil
}

func (m MapLedger) Touch(owner, controller, key string) {
	m.RegisterTouches[string(RegisterID(owner, controller, key))] = true
}

func (m MapLedger) Delete(owner, controller, key string) {
	delete(m.Registers, string(RegisterID(owner, controller, key)))
}

func RegisterID(owner, controller, key string) flow.RegisterID {
	return fullKeyHash(owner, controller, key)
}

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}

func fullKeyHash(owner, controller, key string) flow.RegisterID {
	hasher := hash.NewSHA2_256()
	return flow.RegisterID(hasher.ComputeHash([]byte(fullKey(owner, controller, key))))
}
