package types

// A LedgerDelta is a record of ledger mutations.
type LedgerDelta map[string][]byte

// NewLedgerDelta returns an empty ledger delta.
func NewLedgerDelta() LedgerDelta {
	return make(map[string][]byte)
}

// Get reads a register value from this delta.
//
// This function will return nil if the given key has been deleted in this delta.
func (d LedgerDelta) Get(key string) (value []byte, exists bool) {
	value, exists = d[key]
	return
}

// Set records an update in this delta.
func (d LedgerDelta) Set(key string, value []byte) {
	d[key] = value
}

// Delete records a deletion in this delta.
func (d LedgerDelta) Delete(key string) {
	d[key] = nil
}

// Updates returns all registers that were updated by this delta.
func (d LedgerDelta) Updates() map[string][]byte {
	return d
}

// HasBeenDeleted returns true if the given key has been deleted in this delta.
func (d LedgerDelta) HasBeenDeleted(key string) bool {
	value, exists := d.Get(key)
	return exists && value == nil
}

// GetRegisterFunc is a function that returns the value for a register.
type GetRegisterFunc func(key string) ([]byte, error)

// A LedgerView is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type LedgerView struct {
	delta    LedgerDelta
	readFunc GetRegisterFunc
}

// NewLedgerView instantiates a new ledger view with the provided read function.
func NewLedgerView(readFunc GetRegisterFunc) *LedgerView {
	return &LedgerView{
		delta:    NewLedgerDelta(),
		readFunc: readFunc,
	}
}

// Get gets a register value from this view.
//
// This function will return an error if it fails to read from the underlying
// data source for this view.
func (r *LedgerView) Get(key string) ([]byte, error) {
	value, exists := r.delta.Get(key)
	if exists {
		return value, nil
	}

	value, err := r.readFunc(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Set sets a register value in this view.
func (r *LedgerView) Set(key string, value []byte) {
	r.delta.Set(key, value)
}

// Delete removes a register in this view.
func (r *LedgerView) Delete(key string) {
	r.delta.Delete(key)
}

// Delta returns a record of the registers that were mutated in this view.
func (r *LedgerView) Delta() LedgerDelta {
	return r.delta
}
