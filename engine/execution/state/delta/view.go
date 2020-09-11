package delta

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

// GetRegisterFunc is a function that returns the value for a register.
type GetRegisterFunc func(owner, controller, key string) (flow.RegisterValue, error)

// A View is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type View struct {
	delta       Delta
	regTouchSet map[string]bool // contains all the registers that have been touched (either read or written to)
	readsCount  uint64          // contains the total number of reads
	// SpocksSecret keeps the secret used for SPoCKs
	// TODO we can add a flag to disable capturing SpocksSecret
	// for views other than collection views to improve performance
	spockSecret []byte
	readFunc    GetRegisterFunc
}

// Snapshot is set of interactions with the register
type Snapshot struct {
	Delta       Delta
	Reads       []flow.RegisterID
	SpockSecret []byte
}

// NewView instantiates a new ledger view with the provided read function.
func NewView(readFunc GetRegisterFunc) *View {
	return &View{
		delta:       NewDelta(),
		regTouchSet: make(map[string]bool),
		spockSecret: make([]byte, 0),
		readFunc:    readFunc,
	}
}

// Snapshot returns copy of current state of interactions with a View
func (r *View) Interactions() *Snapshot {

	var delta = Delta{
		Data:          make(map[string]flow.RegisterValue, len(r.delta.Data)),
		WriteMappings: make(map[string]Mapping),
		ReadMappings:  make(map[string]Mapping),
	}
	var reads = make([]flow.RegisterID, 0, len(r.regTouchSet))
	var spockSecret = make([]byte, len(r.spockSecret))

	//copy data
	for s, value := range r.delta.Data {
		delta.Data[s] = value
	}

	for k, v := range r.delta.ReadMappings {
		delta.ReadMappings[k] = v
	}
	for k, v := range r.delta.WriteMappings {
		delta.WriteMappings[k] = v
	}
	for key := range r.regTouchSet {
		reads = append(reads, []byte(key))
	}

	copy(spockSecret, r.spockSecret)

	return &Snapshot{
		Delta:       delta,
		Reads:       reads,
		SpockSecret: spockSecret,
	}
}

// AllRegisters returns all the register IDs either in read or delta
func (r *Snapshot) AllRegisters() []flow.RegisterID {
	set := make(map[string]bool, len(r.Reads)+len(r.Delta.Data))
	for _, reg := range r.Reads {
		set[string(reg)] = true
	}
	for _, reg := range r.Delta.RegisterIDs() {
		set[string(reg)] = true
	}
	ret := make([]flow.RegisterID, 0, len(set))
	for r := range set {
		ret = append(ret, flow.RegisterID(r))
	}
	return ret
}

// NewChild generates a new child view, with the current view as the base, sharing the Get function
func (v *View) NewChild() *View {
	return NewView(v.Get)
}

// Get gets a register value from this view.
//
// This function will return an error if it fails to read from the underlying
// data source for this view.
func (v *View) Get(owner, controller, key string) (flow.RegisterValue, error) {
	value, exists := v.delta.Get(owner, controller, key)
	if exists {
		// every time we read a value (order preserving) we update spock
		err := v.updateSpock(value)
		return value, err
	}

	value, err := v.readFunc(owner, controller, key)
	if err != nil {
		return nil, err
	}

	// capture register touch
	v.regTouchSet[string(state.RegisterID(owner, controller, key))] = true

	// increase reads
	v.readsCount++

	// every time we read a value (order preserving) we update spock
	err = v.updateSpock(value)
	return value, err
}

// Set sets a register value in this view.
func (v *View) Set(owner, controller, key string, value flow.RegisterValue) {
	// every time we write something to delta (order preserving) we update spock
	// TODO return the error and handle it properly on other places
	err := v.updateSpock(value)
	if err != nil {
		panic(err)
	}

	// capture register touch
	k := state.RegisterID(owner, controller, key)

	v.regTouchSet[string(k)] = true
	// add key value to delta
	v.delta.Set(owner, controller, key, value)
}

func (v *View) updateSpock(value []byte) error {
	hasher := hash.NewSHA3_256()
	_, err := hasher.Write(v.spockSecret)
	if err != nil {
		return fmt.Errorf("error updating spock secret data (step1): %w", err)
	}
	_, err = hasher.Write(value)
	if err != nil {
		return fmt.Errorf("error updating spock secret data (step2): %w", err)
	}
	v.spockSecret = hasher.SumHash()
	return nil
}

// Touch explicitly adds a register to the touched registers set.
func (v *View) Touch(owner, controller, key string) {

	k := state.RegisterID(owner, controller, key)

	// capture register touch
	v.regTouchSet[string(k)] = true
	// increase reads
	v.readsCount++
}

// Delete removes a register in this view.
func (v *View) Delete(owner, controller, key string) {
	v.delta.Delete(owner, controller, key)
}

// Delta returns a record of the registers that were mutated in this view.
func (v *View) Delta() Delta {
	return v.delta
}

// MergeView applies the changes from a the given view to this view.
// TODO rename this, this is not actually a merge as we can't merge
// readFunc s.
func (v *View) MergeView(child *View) {
	for _, k := range child.Interactions().RegisterTouches() {
		v.regTouchSet[string(k)] = true
	}
	// SpockSecret is order aware
	// TODO return the error and handle it properly on other places
	err := v.updateSpock(child.spockSecret)
	if err != nil {
		panic(err)
	}
	v.delta.MergeWith(child.delta)
}

// RegisterTouches returns the register IDs touched by this view (either read or write)
func (r *Snapshot) RegisterTouches() []flow.RegisterID {
	ret := make([]flow.RegisterID, 0, len(r.Reads))
	ret = append(ret, r.Reads...)
	return ret
}

// ReadsCount returns the total number of reads performed on this view including all child views
func (r *View) ReadsCount() uint64 {
	return r.readsCount
}

// SpockSecret returns the secret value for SPoCK
func (v *View) SpockSecret() []byte {
	return v.spockSecret
}
