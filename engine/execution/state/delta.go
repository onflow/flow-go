package state

import "github.com/dapperlabs/flow-go/model/flow"

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

// RegisterUpdates returns all registers that were updated by this delta.
func (d Delta) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	ids := make([]flow.RegisterID, 0, len(d))
	values := make([]flow.RegisterValue, 0, len(d))

	for id, value := range d {
		ids = append(ids, flow.RegisterID(id))
		values = append(values, value)
	}

	return ids, values
}

func (d Delta) RegisterDelta() flow.RegisterDelta {
	return flow.RegisterDelta(d)
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
