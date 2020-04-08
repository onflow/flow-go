package state

import "github.com/dapperlabs/flow-go/model/flow"

// GetRegisterFunc is a function that returns the value for a register.
type GetRegisterFunc func(key flow.RegisterID) (flow.RegisterValue, error)

// A View is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type View struct {
	delta Delta
	// TODO: store reads as set
	reads []flow.RegisterID
	// SpocksSecret keeps the secret used for SPoCKs
	// TODO we can add a flag to disable capturing SpocksSecret
	// for views other than collection views to improve performance
	spockSecret []byte
	readFunc    GetRegisterFunc
}

// NewView instantiates a new ledger view with the provided read function.
func NewView(readFunc GetRegisterFunc) *View {
	return &View{
		delta:       NewDelta(),
		reads:       make([]flow.RegisterID, 0),
		spockSecret: make([]byte, 0),
		readFunc:    readFunc,
	}
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
		return value, nil
	}

	value, err := v.readFunc(key)
	if err != nil {
		return nil, err
	}

	// record read
	v.reads = append(v.reads, key)

	return value, nil
}

// Set sets a register value in this view.
func (v *View) Set(key flow.RegisterID, value flow.RegisterValue) {
	// every time we write something to delta (order preserving)
	// we append the value to the end of the SpocksSecret byte slice
	v.spockSecret = append(v.spockSecret, value...)
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
func (v *View) MergeView(child *View) {
	v.spockSecret = append(v.spockSecret, child.spockSecret...)
	v.delta.MergeWith(child.delta)
}

// Reads returns the register IDs read by this view.
func (v *View) Reads() []flow.RegisterID {
	return v.reads
}

// AllRegisters returns all the register IDs either in read or delta
func (v *View) AllRegisters() []flow.RegisterID {
	set := make(map[string]bool, len(v.reads)+len(v.delta))
	for _, reg := range v.reads {
		set[string(reg)] = true
	}
	for _, reg := range v.delta.RegisterIDs() {
		set[string(reg)] = true
	}
	ret := make([]flow.RegisterID, 0, len(set))
	for r := range set {
		ret = append(ret, flow.RegisterID(r))
	}
	return ret
}

// SpockSecret returns the secret value for SPoCK
func (v *View) SpockSecret() []byte {
	return v.spockSecret
}
