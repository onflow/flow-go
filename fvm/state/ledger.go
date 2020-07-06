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
	RegTouchSet map[string]bool
	Registers   map[string]flow.RegisterValue
}

func (m MapLedger) Set(key flow.RegisterID, value flow.RegisterValue) {
	m.RegTouchSet[string(key)] = true
	m.Registers[string(key)] = value
}

func (m MapLedger) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	m.RegTouchSet[string(key)] = true
	return m.Registers[string(key)], nil
}

func (m MapLedger) Touch(key flow.RegisterID) {
	m.RegTouchSet[string(key)] = true
}

func (m MapLedger) Delete(key flow.RegisterID) {
	delete(m.Registers, string(key))
}

const (
	keyAddressState   = "account_address_state"
	keyUUID           = "uuid"
	keyExists         = "exists"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
)

func fullKey(owner, controller, key string) string {
	// https://en.wikipedia.org/wiki/C0_and_C1_control_codes#Field_separators
	return strings.Join([]string{owner, controller, key}, "\x1F")
}

func fullKeyHash(owner, controller, key string) flow.RegisterID {
	h := sha256.New()
	_, _ = h.Write([]byte(fullKey(owner, controller, key)))
	return h.Sum(nil)
}
