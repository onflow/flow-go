package delta

import (
	"bytes"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
)

// A Delta is a record of ledger mutations.
type Delta map[string]flow.RegisterValue

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return make(map[string]flow.RegisterValue)
}

// Get reads a register value from this delta.
//
// This function will return nil if the given key has been deleted in this delta.
func (d Delta) Get(key flow.RegisterID) (value flow.RegisterValue, exists bool) {
	value, exists = d[string(key)]
	return
}

// Set records an update in this delta.
func (d Delta) Set(key flow.RegisterID, value flow.RegisterValue) {
	d[string(key)] = value
}

// Delete records a deletion in this delta.
func (d Delta) Delete(key flow.RegisterID) {
	d[string(key)] = nil
}

// RegisterUpdates returns all registers that were updated by this delta.
// ids are returned sorted, in ascending order
func (d Delta) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	ids := make([]flow.RegisterID, 0, len(d))
	values := make([]flow.RegisterValue, 0, len(d))

	for id := range d {
		ids = append(ids, flow.RegisterID(id))
	}

	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i], ids[j]) < 0
	})

	for _, id := range ids {
		values = append(values, d[string(id)])
	}

	return ids, values
}

// HasBeenDeleted returns true if the given key has been deleted in this delta.
func (d Delta) HasBeenDeleted(key flow.RegisterID) bool {
	value, exists := d[string(key)]
	return exists && value == nil
}

// MergeWith merges this delta with another.
func (d Delta) MergeWith(delta Delta) {
	for key, value := range delta {
		d[key] = value
	}
}

// RegisterIDs returns the list of registerIDs inside this delta
func (d Delta) RegisterIDs() []flow.RegisterID {
	ids := make([]flow.RegisterID, 0, len(d))
	for id := range d {
		ids = append(ids, flow.RegisterID(id))
	}
	return ids
}
