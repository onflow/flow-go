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
func (r *View) NewChild() *View {
	return NewView(r.Get)
}

// Get gets a register value from this view.
//
// This function will return an error if it fails to read from the underlying
// data source for this view.
func (r *View) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	value, exists := r.delta.Get(key)
	if exists {
		return value, nil
	}

	value, err := r.readFunc(key)
	if err != nil {
		return nil, err
	}

	// record read
	r.reads = append(r.reads, key)

	return value, nil
}

// Set sets a register value in this view.
func (r *View) Set(key flow.RegisterID, value flow.RegisterValue) {
	// every time we write something to delta (order preserving)
	// we append the value to the end of the SpocksSecret byte slice
	r.spockSecret = append(r.spockSecret, value...)
	r.delta.Set(key, value)
}

// Delete removes a register in this view.
func (r *View) Delete(key flow.RegisterID) {
	r.delta.Delete(key)
}

// Delta returns a record of the registers that were mutated in this view.
func (r *View) Delta() Delta {
	return r.delta
}

// ApplyDelta applies the changes from a delta to this view.
func (r *View) ApplyDelta(delta Delta) {
	r.delta.MergeWith(delta)
}

// Reads returns the register IDs read by this view.
func (r *View) Reads() []flow.RegisterID {
	return r.reads
}

// AllRegisters returns all the register IDs either in read or delta
func (r *View) AllRegisters() []flow.RegisterID {
	set := make(map[string]bool, len(r.reads)+len(r.delta))
	for _, reg := range r.reads {
		set[string(reg)] = true
	}
	for _, reg := range r.delta.RegisterIDs() {
		set[string(reg)] = true
	}
	ret := make([]flow.RegisterID, 0, len(set))
	for r := range set {
		ret = append(ret, flow.RegisterID(r))
	}
	return ret
}

// SpockSecret returns the secret value for SPoCK
func (r *View) SpockSecret() []byte {
	return r.spockSecret
}
