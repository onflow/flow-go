package state

// GetRegisterFunc is a function that returns the value for a register.
type GetRegisterFunc func(key string) ([]byte, error)

// A View is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type View struct {
	delta    Delta
	readFunc GetRegisterFunc
}

// NewView instantiates a new ledger view with the provided read function.
func NewView(readFunc GetRegisterFunc) *View {
	return &View{
		delta:    NewDelta(),
		readFunc: readFunc,
	}
}

func (r *View) NewChild() *View {
	return NewView(r.Get)
}

// Get gets a register value from this view.
//
// This function will return an error if it fails to read from the underlying
// data source for this view.
func (r *View) Get(key string) ([]byte, error) {
	value, exists := r.delta.Get(key)
	if exists {
		return value, nil
	}

	value, err := r.readFunc(key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Set sets a register value in this view.
func (r *View) Set(key string, value []byte) {
	r.delta.Set(key, value)
}

// Delete removes a register in this view.
func (r *View) Delete(key string) {
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
