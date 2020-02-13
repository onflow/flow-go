package virtualmachine

// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(key string, value []byte)
	Get(key string) ([]byte, error)
	Delete(key string)
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type MapLedger map[string][]byte

func (m MapLedger) Set(key string, value []byte) {
	m[key] = value
}

func (m MapLedger) Get(key string) ([]byte, error) {
	return m[key], nil
}

func (m MapLedger) Delete(key string) {
	delete(m, key)
}
