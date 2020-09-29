package delta

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/onflow/flow-go/model/flow"
)

// A Delta is a record of ledger mutations.
type Delta struct {
	Data map[string]flow.RegisterEntry
}

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return Delta{
		Data: make(map[string]flow.RegisterEntry),
	}
}

func toString(owner, controller, key string) string {
	register := toRegisterID(owner, controller, key)
	return register.String()
}

func toRegisterID(owner, controller, key string) flow.RegisterID {
	return flow.RegisterID{
		Owner:      owner,
		Controller: controller,
		Key:        key,
	}
}

// func fromString(key string) flow.RegisterID {
// 	return []byte(key)
// }

// Get reads a register value from this delta.
//
// This function will return nil if the given key has been deleted in this delta.
// Second return parameters indicated if the value has been set/deleted in this delta
func (d Delta) Get(owner, controller, key string) (flow.RegisterValue, bool) {
	value, set := d.Data[toString(owner, controller, key)]
	return value.Value, set
}

// Set records an update in this delta.
func (d Delta) Set(owner, controller, key string, value flow.RegisterValue) {
	k := toString(owner, controller, key)
	d.Data[k] = flow.RegisterEntry{
		Key:   toRegisterID(owner, controller, key),
		Value: value,
	}
}

// Delete records a deletion in this delta.
func (d Delta) Delete(owner, controller, key string) {
	k := toString(owner, controller, key)
	d.Data[k] = flow.RegisterEntry{
		Key:   toRegisterID(owner, controller, key),
		Value: nil,
	}
}

// RegisterUpdates returns all registers that were updated by this delta.
// ids are returned sorted, in ascending order
func (d Delta) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {

	data := make(flow.RegisterEntries, 0, len(d.Data))

	for _, v := range d.Data {
		data = append(data, v)
	}

	sort.Sort(&data)

	ids := make([]flow.RegisterID, 0, len(d.Data))
	values := make([]flow.RegisterValue, 0, len(d.Data))

	for _, v := range data {
		ids = append(ids, v.Key)
		values = append(values, v.Value)
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
	for _, v := range d.Data {
		ids = append(ids, v.Key)
	}
	return ids
}

func (d Delta) MarshalJSON() ([]byte, error) {
	m := make(flow.RegisterEntries, len(d.Data))
	for _, value := range d.Data {
		m = append(m, value)
	}
	return json.Marshal(m)
}

func (d Delta) UnmarshalJSON(data []byte) error {

	var m flow.RegisterEntries

	err := json.Unmarshal(data, &m)
	if err != nil {
		return fmt.Errorf("cannot umarshal Delta: %w", err)
	}
	dd := make(map[string]flow.RegisterEntry, len(m))

	for _, value := range m {
		dd[value.Key.String()] = value
	}

	d.Data = dd

	return nil
}
