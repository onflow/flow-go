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
	state flow.StateCommitment
	// TODO: store reads as set
	reads    []flow.RegisterID
	readFunc GetRegisterFunc
}

// NewView instantiates a new ledger view with the provided read function.
func NewView(readFunc GetRegisterFunc, state flow.StateCommitment) *View {
	return &View{
		delta:    NewDelta(),
		state:    state,
		reads:    make([]flow.RegisterID, 0),
		readFunc: readFunc,
	}
}

// NewChild generates a new child view, with the current view as the base, sharing the Get function
func (r *View) NewChild() *View {
	return NewView(r.Get, r.state)
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
