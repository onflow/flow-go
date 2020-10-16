package delta

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/onflow/flow-go/model/flow"
)

// TODO Remove after migration
type Mapping struct {
	Owner      string
	Key        string
	Controller string
}

type MappingForJson struct {
	Owner      string
	Key        string
	Controller string
}

func (m Mapping) MarshalJSON() ([]byte, error) {
	return json.Marshal(MappingForJson{
		Owner:      hex.EncodeToString([]byte(m.Owner)),
		Key:        hex.EncodeToString([]byte(m.Key)),
		Controller: hex.EncodeToString([]byte(m.Controller)),
	})
}

func (m *Mapping) UnmarshalJSON(data []byte) error {
	var mm MappingForJson
	err := json.Unmarshal(data, &mm)
	if err != nil {
		return err
	}
	dOwner, err := hex.DecodeString(mm.Owner)
	if err != nil {
		return fmt.Errorf("cannot decode owner: %w", err)
	}
	dController, err := hex.DecodeString(mm.Controller)
	if err != nil {
		return fmt.Errorf("cannot decode controller: %w", err)
	}
	dKey, err := hex.DecodeString(mm.Key)
	if err != nil {
		return fmt.Errorf("cannot decode key: %w", err)
	}

	*m = Mapping{
		Owner:      string(dOwner),
		Key:        string(dKey),
		Controller: string(dController),
	}
	return nil
}

type LegacyDelta struct {
	Data          map[string]flow.RegisterValue
	ReadMappings  map[string]Mapping // kept for Ledger migration only
	WriteMappings map[string]Mapping // kept for Ledger migration only
}

type LegacySnapshot struct {
	Delta       LegacyDelta
	Reads       []flow.LegacyRegisterID
	SpockSecret []byte
}

// End of removal

// A Delta is a record of ledger mutations.
type Delta struct {
	Data               map[string]flow.RegisterEntry
}

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return Delta{
		Data:               make(map[string]flow.RegisterEntry),
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
	r := flow.RegisterEntry{
		Key:   toRegisterID(owner, controller, key),
		Value: value,
	}

	d.Data[k] = r
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
