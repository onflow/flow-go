package virtualmachine

import "github.com/dapperlabs/flow-go/model/flow"

// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(key flow.RegisterID, value flow.RegisterValue)
	Get(key flow.RegisterID) (flow.RegisterValue, error)
	Delete(key flow.RegisterID)
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type MapLedger map[string]flow.RegisterValue

func (m MapLedger) Set(key flow.RegisterID, value flow.RegisterValue) {
	m[string(key)] = value
}

func (m MapLedger) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	return m[string(key)], nil
}

func (m MapLedger) Delete(key flow.RegisterID) {
	delete(m, string(key))
}
