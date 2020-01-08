package ledger

// A Delta is a record of ledger mutations.
type Delta map[string][]byte

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return make(map[string][]byte)
}

// Get reads a register value from this delta.
//
// This function will return nil if the given key has been deleted in this delta.
func (d Delta) Get(key string) (value []byte, exists bool) {
	value, exists = d[key]
	return
}

// Set records an update in this delta.
func (d Delta) Set(key string, value []byte) {
	d[key] = value
}

// Delete records a deletion in this delta.
func (d Delta) Delete(key string) {
	d[key] = nil
}

// Updates returns all registers that were updated by this delta.
func (d Delta) Updates() map[string][]byte {
	return d
}

// HasBeenDeleted returns true if the given key has been deleted in this delta.
func (d Delta) HasBeenDeleted(key string) bool {
	value, exists := d[key]
	return exists && value == nil
}

// MergeWith merges this delta with another.
func (d Delta) MergeWith(delta Delta) {
	for key, value := range delta {
		d[key] = value
	}
}
