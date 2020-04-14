package delta

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
)

// A Delta is a record of ledger mutations.
type Delta map[string]flow.RegisterValue

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return make(map[string]flow.RegisterValue)
}

// store
func toString(key []byte) string {
	return string(key)
}

func fromString(key string) flow.RegisterID {
	return []byte(key)
}

// Get reads a register value from this delta.
//
// This function will return nil if the given key has been deleted in this delta.
func (d Delta) Get(key flow.RegisterID) (value flow.RegisterValue, exists bool) {
	value, exists = d[toString(key)]
	return
}

// Set records an update in this delta.
func (d Delta) Set(key flow.RegisterID, value flow.RegisterValue) {
	d[toString(key)] = value
}

// Delete records a deletion in this delta.
func (d Delta) Delete(key flow.RegisterID) {
	d[toString(key)] = nil
}

//handy container for sorting
type idsValues struct {
	ids    []flow.RegisterID
	values []flow.RegisterValue
}

func (d *idsValues) Len() int {
	return len(d.ids)
}

func (d *idsValues) Less(i, j int) bool {
	return bytes.Compare(d.ids[i], d.ids[j]) < 0
}

func (d *idsValues) Swap(i, j int) {
	d.ids[i], d.ids[j] = d.ids[j], d.ids[i]
	d.values[i], d.values[j] = d.values[j], d.values[i]
}

// RegisterUpdates returns all registers that were updated by this delta.
// ids are returned sorted, in ascending order
func (d Delta) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {

	data := idsValues{
		ids:    make([]flow.RegisterID, 0, len(d)),
		values: make([]flow.RegisterValue, 0, len(d)),
	}

	for id := range d {
		data.ids = append(data.ids, fromString(id))
		data.values = append(data.values, d[id])
	}

	sort.Sort(&data)

	return data.ids, data.values
}

// HasBeenDeleted returns true if the given key has been deleted in this delta.
func (d Delta) HasBeenDeleted(key flow.RegisterID) bool {
	value, exists := d[toString(key)]
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

func (d Delta) MarshalJSON() ([]byte, error) {
	m := make(map[string]flow.RegisterValue, len(d))
	for key, value := range d {
		hexKey := hex.EncodeToString(fromString(key))
		m[hexKey] = value
	}
	return json.Marshal(m)
}

func (d *Delta) UnmarshalJSON(data []byte) error {

	m := make(map[string]flow.RegisterValue)

	dd := make(map[string]flow.RegisterValue)

	err := json.Unmarshal(data, &m)
	if err != nil {
		return fmt.Errorf("cannot umarshal Delta: %w", err)
	}

	for key, value := range m {
		bytesKey, err := hex.DecodeString(key)
		if err != nil {
			return fmt.Errorf("cannot decode key for Delta (%s): %w", key, err)
		}
		dd[toString(bytesKey)] = value

	}

	*d = dd

	return nil
}
