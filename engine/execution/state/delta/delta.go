package delta

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/vmihailenco/msgpack/v4"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

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

// A Delta is a record of ledger mutations.
type Delta struct {
	Data          map[string]flow.RegisterValue
	ReadMappings  map[string]Mapping
	WriteMappings map[string]Mapping
}

func (b *Delta) MarshalBSON() ([]byte, error) {
	data := make(map[string]flow.RegisterValue, len(b.Data))
	read := make(map[string]Mapping, len(b.ReadMappings))
	write := make(map[string]Mapping, len(b.WriteMappings))

	for k, v := range b.Data {
		data[hex.EncodeToString([]byte(k))] = v
	}
	for k, v := range b.ReadMappings {
		read[hex.EncodeToString([]byte(k))] = v
	}
	for k, v := range b.WriteMappings {
		write[hex.EncodeToString([]byte(k))] = v
	}

	return bson.Marshal(Delta{
		Data:          data,
		ReadMappings:  read,
		WriteMappings: write,
	})
}

// NewDelta returns an empty ledger delta.
func NewDelta() Delta {
	return Delta{
		Data:          make(map[string]flow.RegisterValue),
		ReadMappings:  make(map[string]Mapping),
		WriteMappings: make(map[string]Mapping),
	}
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
func (d Delta) Get(owner, controller, key string) (value flow.RegisterValue, exists bool) {
	k := state.RegisterID(owner, controller, key)
	d.ReadMappings[toString(k)] = Mapping{
		Owner:      owner,
		Controller: controller,
		Key:        key,
	}
	if controller == "" {
		d.ReadMappings[toString(k)] = Mapping{
			Owner:      owner,
			Controller: owner,
			Key:        key,
		}
	}
	value, exists = d.Data[toString(k)]
	return
}

// Set records an update in this delta.
func (d Delta) Set(owner, controller, key string, value flow.RegisterValue) {
	k := toString(state.RegisterID(owner, controller, key))
	d.WriteMappings[k] = Mapping{
		Owner:      owner,
		Controller: controller,
		Key:        key,
	}
	if controller == "" {
		d.WriteMappings[k] = Mapping{
			Owner:      owner,
			Controller: owner,
			Key:        key,
		}
	}
	d.Data[k] = value
}

// Delete records a deletion in this delta.
func (d Delta) Delete(owner, controller, key string) {
	k := toString(state.RegisterID(owner, controller, key))
	d.Data[k] = nil
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
		ids:    make([]flow.RegisterID, 0, len(d.Data)),
		values: make([]flow.RegisterValue, 0, len(d.Data)),
	}

	for id := range d.Data {
		data.ids = append(data.ids, fromString(id))
		data.values = append(data.values, d.Data[id])
	}

	sort.Sort(&data)

	return data.ids, data.values
}

// HasBeenDeleted returns true if the given key has been deleted in this delta.
func (d Delta) HasBeenDeleted(key flow.RegisterID) bool {
	value, exists := d.Data[toString(key)]
	return exists && value == nil
}

// MergeWith merges this delta with another.
func (d Delta) MergeWith(delta Delta) {
	for key, value := range delta.Data {
		d.Data[key] = value
	}
	for key, value := range delta.ReadMappings {
		d.ReadMappings[key] = value
	}

	for key, value := range delta.WriteMappings {
		d.WriteMappings[key] = value
	}
}

// RegisterIDs returns the list of registerIDs inside this delta
func (d Delta) RegisterIDs() []flow.RegisterID {
	ids := make([]flow.RegisterID, 0, len(d.Data))
	for id := range d.Data {
		ids = append(ids, flow.RegisterID(id))
	}
	return ids
}

func (d Delta) MarshalJSON() ([]byte, error) {
	m := make(map[string]flow.RegisterValue, len(d.Data))
	for key, value := range d.Data {
		hexKey := hex.EncodeToString(fromString(key))
		m[hexKey] = value
	}
	return json.Marshal(m)
}

func (d Delta) UnmarshalJSON(data []byte) error {

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

	d.Data = dd
	d.ReadMappings = make(map[string]Mapping)
	d.WriteMappings = make(map[string]Mapping)

	return nil
}

func (d *Delta) DecodeMsgpack(decoder *msgpack.Decoder) error {

	var m map[string]flow.RegisterValue

	err := decoder.Decode(&m)

	dd := make(map[string]flow.RegisterValue)

	if err != nil {
		return fmt.Errorf("cannot umarshal Delta: %w", err)
	}

	for key, value := range m {
		//bytesKey, err := hex.DecodeString(key)
		//if err != nil {
		//	return fmt.Errorf("cannot decode key for Delta (%s): %w", key, err)
		//}
		//dd[toString(bytesKey)] = value

		dd[key] = value

	}

	*d = Delta{
		Data:          dd,
		ReadMappings:  make(map[string]Mapping),
		WriteMappings: make(map[string]Mapping),
	}

	return nil
}
