package delta

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// GetRegisterFunc is a function that returns the value for a register.
type GetRegisterFunc func(key flow.RegisterID) (flow.RegisterValue, error)

// A View is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type View struct {
	delta       Delta
	regTouchSet map[string]bool
	// SpocksSecret keeps the secret used for SPoCKs
	// TODO we can add a flag to disable capturing SpocksSecret
	// for views other than collection views to improve performance
	spockSecret []byte
	readFunc    GetRegisterFunc
}

// Snapshot is set of interactions with the register
type Snapshot struct {
	Delta Delta
	Reads []flow.RegisterID
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

	var delta = make(Delta, len(r.delta))
	var reads = make([]flow.RegisterID, 0, len(r.regTouchSet))

	//copy data
	for s, value := range r.delta {
		delta[s] = value
	}
	for key, _ := range r.regTouchSet {
		reads = append(reads, []byte(key))
	}

	return &Snapshot{
		Delta: delta,
		Reads: reads,
	}
}

// AllRegisters returns all the register IDs ether in read or delta
func (r *Snapshot) AllRegisters() []flow.RegisterID {
	set := make(map[string]bool, len(r.Reads)+len(r.Delta))
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
func (v *View) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	value, exists := v.delta.Get(key)
	if exists {
		// every time we read a value (order preserving)
		// we append the value to the end of the SpocksSecret byte slice
		v.spockSecret = append(v.spockSecret, value...)
		return value, nil
	}

	value, err := v.readFunc(key)
	if err != nil {
		return nil, err
	}

	// capture register touch
	v.regTouchSet[string(key)] = true

	// every time we read a value (order preserving)
	// we append the value to the end of the SpocksSecret byte slice
	v.spockSecret = append(v.spockSecret, value...)
	return value, nil
}

// Set sets a register value in this view.
func (v *View) Set(key flow.RegisterID, value flow.RegisterValue) {
	// every time we write something to delta (order preserving)
	// we append the value to the end of the SpocksSecret byte slice
	v.spockSecret = append(v.spockSecret, value...)
	// capture register touch
	v.regTouchSet[string(key)] = true
	// add key value to delta
	v.delta.Set(key, value)
}

// Delete removes a register in this view.
func (v *View) Delete(key flow.RegisterID) {
	v.delta.Delete(key)
}

// Delta returns a record of the registers that were mutated in this view.
func (v *View) Delta() Delta {
	return v.delta
}

// MergeView applies the changes from a the given view to this view.
// TODO rename this, this is not actually a merge as we can't merge
// readFunc s.
func (v *View) MergeView(child *View) {
	for k := range child.Interactions().RegisterTouches() {
		v.regTouchSet[string(k)] = true
	}
	// SpockSecret is order aware
	v.spockSecret = append(v.spockSecret, child.spockSecret...)
	v.delta.MergeWith(child.delta)
}



// RegisterTouches returns the register IDs touched by this view (either read or write)
func (r *Snapshot) RegisterTouches() []flow.RegisterID {
	ret := make([]flow.RegisterID, 0, len(r.Reads))
	for _, r := range r.Reads {
		ret = append(ret, r)
	}
	return ret
}

// SpockSecret returns the secret value for SPoCK
func (v *View) SpockSecret() []byte {
	return v.spockSecret
}
