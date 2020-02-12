package virtualmachine

// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(key string, value []byte)
	Get(key string) ([]byte, error)
	Delete(key string)
}
