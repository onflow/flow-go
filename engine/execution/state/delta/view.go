package delta

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// GetRegisterFunc is a function that returns the value for a register.
type GetRegisterFunc func(owner, controller, key string) (flow.RegisterValue, error)

// A View is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type View struct {
	delta       Delta
	regTouchSet map[string]flow.RegisterID // contains all the registers that have been touched (either read or written to)
	readsCount  uint64                     // contains the total number of reads
	// spockSecret keeps the secret used for SPoCKs
	// TODO we can add a flag to disable capturing spockSecret
	// for views other than collection views to improve performance
	spockSecret       []byte
	spockSecretLock   sync.Mutex
	spockSecretHasher hash.Hasher
	readFunc          GetRegisterFunc
}

type Snapshot struct {
	Delta Delta
	Reads map[string]flow.RegisterID
}

// Snapshot is state of interactions with the register
type SpockSnapshot struct {
	Snapshot
	SpockSecret []byte
}

func AlwaysEmptyGetRegisterFunc(owner, controller, key string) (flow.RegisterValue, error) {
	return nil, nil
}

// NewView instantiates a new ledger view with the provided read function.
func NewView(readFunc GetRegisterFunc) *View {
	return &View{
		delta:             NewDelta(),
		regTouchSet:       make(map[string]flow.RegisterID),
		readFunc:          readFunc,
		spockSecretHasher: hash.NewSHA3_256(),
	}
}

// Snapshot returns copy of current state of interactions with a View
func (v *View) Interactions() *SpockSnapshot {

	var delta = Delta{
		Data: make(map[string]flow.RegisterEntry, len(v.delta.Data)),
	}
	var reads = make(map[string]flow.RegisterID, len(v.regTouchSet))

	//copy data
	for s, value := range v.delta.Data {
		delta.Data[s] = value
	}

	for i, id := range v.regTouchSet {
		reads[i] = id
	}

	return &SpockSnapshot{
		Snapshot: Snapshot{
			Delta: delta,
			Reads: reads,
		},
		SpockSecret: v.SpockSecret(),
	}
}

// AllRegisters returns all the register IDs either in read or delta
func (r *Snapshot) AllRegisters() []flow.RegisterID {
	set := make(map[string]flow.RegisterID, len(r.Reads)+len(r.Delta.Data))
	for _, reg := range r.Reads {
		set[reg.String()] = reg
	}
	for _, reg := range r.Delta.RegisterIDs() {
		set[reg.String()] = reg
	}
	ret := make([]flow.RegisterID, 0, len(set))
	for _, r := range set {
		ret = append(ret, r)
	}
	return ret
}

// NewChild generates a new child view, with the current view as the base, sharing the Get function
func (v *View) NewChild() state.View {
	return NewView(v.Peek)
}

func (v *View) DropDelta() {
	v.delta = NewDelta()
}

func (v *View) AllRegisters() []flow.RegisterID {
	return v.Interactions().AllRegisters()
}

func (v *View) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	return v.Delta().RegisterUpdates()
}

// Get gets a register value from this view.
//
// This function will return an error if it fails to read from the underlying
// data source for this view.
func (v *View) Get(owner, controller, key string) (flow.RegisterValue, error) {
	var err error
	registerID := flow.NewRegisterID(owner, controller, key)

	value, exists := v.delta.Get(owner, controller, key)
	if !exists {
		value, err = v.readFunc(owner, controller, key)
		if err != nil {
			return nil, fmt.Errorf("get register failed: %w", err)
		}
		// capture register touch
		v.regTouchSet[registerID.String()] = registerID
		// increase reads
		v.readsCount++
	}
	// every time we read a value (order preserving) we update the secret
	// with the registerID only (value is not required)
	_, err = v.spockSecretHasher.Write(registerID.Bytes())
	if err != nil {
		return nil, fmt.Errorf("get register failed: %w", err)
	}
	return value, nil
}

// Peek reads the value without registering the read, as when used as parent read function
func (v *View) Peek(owner, controller, key string) (flow.RegisterValue, error) {
	value, exists := v.delta.Get(owner, controller, key)
	if exists {
		return value, nil
	}

	return v.readFunc(owner, controller, key)
}

// Set sets a register value in this view.
func (v *View) Set(owner, controller, key string, value flow.RegisterValue) error {
	registerID := flow.NewRegisterID(owner, controller, key)
	// every time we write something to delta (order preserving) we update
	// the spock secret with both the register ID and value.

	_, err := v.spockSecretHasher.Write(registerID.Bytes())
	if err != nil {
		return fmt.Errorf("set register failed: %w", err)
	}

	_, err = v.spockSecretHasher.Write(value)
	if err != nil {
		return fmt.Errorf("set register failed: %w", err)
	}

	// capture register touch
	v.regTouchSet[registerID.String()] = registerID
	// add key value to delta
	v.delta.Set(owner, controller, key, value)
	return nil
}

// Touch explicitly adds a register to the touched registers set.
func (v *View) Touch(owner, controller, key string) error {

	k := flow.NewRegisterID(owner, controller, key)

	// capture register touch
	v.regTouchSet[k.String()] = k
	// increase reads
	v.readsCount++

	return nil
}

// Delete removes a register in this view.
func (v *View) Delete(owner, controller, key string) error {
	return v.Set(owner, controller, key, nil)
}

// Delta returns a record of the registers that were mutated in this view.
func (v *View) Delta() Delta {
	return v.delta
}

// MergeView applies the changes from a the given view to this view.
// TODO rename this, this is not actually a merge as we can't merge
// readFunc s.

func (v *View) MergeView(ch state.View) error {

	child, ok := ch.(*View)
	if !ok {
		return fmt.Errorf("can not merge view: view type mismatch (given: %T, expected:delta.View)", ch)
	}

	for _, id := range child.Interactions().RegisterTouches() {
		v.regTouchSet[id.String()] = id
	}

	// SpockSecret is order aware
	// TODO return the error and handle it properly on other places

	spockSecret := child.SpockSecret()

	_, err := v.spockSecretHasher.Write(spockSecret)
	if err != nil {
		return fmt.Errorf("merging SPoCK secrets failed: %w", err)
	}
	v.delta.MergeWith(child.delta)

	v.readsCount += child.readsCount

	return nil
}

// RegisterTouches returns the register IDs touched by this view (either read or write)
func (r *Snapshot) RegisterTouches() map[string]flow.RegisterID {
	ret := make(map[string]flow.RegisterID, len(r.Reads))
	for k, v := range r.Reads {
		ret[k] = v
	}
	return ret
}

// ReadsCount returns the total number of reads performed on this view including all child views
func (v *View) ReadsCount() uint64 {
	return v.readsCount
}

// SpockSecret returns the secret value for SPoCK
//
// This function modifies the internal state of the SPoCK secret hasher.
// Once called, it doesn't allow writing more data into the SPoCK secret.
func (v *View) SpockSecret() []byte {
	// check if spockSecret has been already computed
	v.spockSecretLock.Lock()
	if v.spockSecret == nil {
		v.spockSecret = v.spockSecretHasher.SumHash()
	}
	v.spockSecretLock.Unlock()
	return v.spockSecret
}

// Detach detaches view from parent, by setting readFunc to
// default, empty one
func (v *View) Detach() {
	v.readFunc = AlwaysEmptyGetRegisterFunc
}
