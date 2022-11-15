package delta

import (
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/flow"
)

// A Delta is a record of ledger mutations.
type Delta struct {
	Data map[flow.RegisterID]flow.RegisterValue
}

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return Delta{
		Data: make(map[flow.RegisterID]flow.RegisterValue),
	}
}

// Get reads a register value from this delta.
//
// This function will return nil if the given key has been deleted in this delta.
// Second return parameters indicated if the value has been set/deleted in this delta
func (d Delta) Get(owner, key string) (flow.RegisterValue, bool) {
	value, set := d.Data[flow.NewRegisterID(owner, key)]
	return value, set
}

// Set records an update in this delta.
func (d Delta) Set(owner, key string, value flow.RegisterValue) {
	k := flow.NewRegisterID(owner, key)
	d.Data[k] = value
}

// RegisterUpdates returns all registers that were updated by this delta.
// ids are returned sorted, in ascending order
func (d Delta) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	ids := make([]flow.RegisterID, 0, len(d.Data))
	for k := range d.Data {
		ids = append(ids, k)
	}

	slices.SortFunc(ids, func(a, b flow.RegisterID) bool {
		return (a.Owner < b.Owner) || (a.Owner == b.Owner && a.Key < b.Key)
	})

	values := make([]flow.RegisterValue, len(d.Data))
	for i, v := range ids {
		values[i] = d.Data[v]
	}

	return ids, values
}

// MergeWith merges this delta with another.
func (d Delta) MergeWith(delta Delta) {
	for key, value := range delta.Data {
		d.Data[key] = value
	}
}

// RegisterIDs returns the list of registerIDs inside this delta
func (d Delta) RegisterIDs() []flow.RegisterID {
	ids := make([]flow.RegisterID, 0, len(d.Data))
	for k := range d.Data {
		ids = append(ids, k)
	}
	return ids
}

func (d Delta) MarshalJSON() ([]byte, error) {
	m := make(flow.RegisterEntries, len(d.Data))
	for key, value := range d.Data {
		m = append(m, flow.RegisterEntry{Key: key, Value: value})
	}
	return json.Marshal(m)
}

func (d *Delta) UnmarshalJSON(data []byte) error {

	var m flow.RegisterEntries

	err := json.Unmarshal(data, &m)
	if err != nil {
		return fmt.Errorf("cannot umarshal Delta: %w", err)
	}
	dd := make(map[flow.RegisterID]flow.RegisterValue, len(m))

	for _, value := range m {
		dd[value.Key] = value.Value
	}

	d.Data = dd

	return nil
}
