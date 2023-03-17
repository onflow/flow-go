package delta

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// A View is a read-only view into a ledger stored in an underlying data source.
//
// A ledger view records writes to a delta that can be used to update the
// underlying data source.
type View struct {
	delta       Delta
	regTouchSet map[flow.RegisterID]struct{} // contains all the registers that have been touched (either read or written to)
	// spockSecret keeps the secret used for SPoCKs
	// TODO we can add a flag to disable capturing spockSecret
	// for views other than collection views to improve performance
	spockSecret       []byte
	spockSecretLock   *sync.Mutex // using pointer instead, because using value would cause mock.Called to trigger race detector
	spockSecretHasher hash.Hasher

	storage state.StorageSnapshot
}

type Snapshot struct {
	Delta Delta
	SnapshotStats
	Reads map[flow.RegisterID]struct{}
}

type SnapshotStats struct {
	NumberOfBytesWrittenToRegisters int
	NumberOfRegistersTouched        int
}

// Snapshot is state of interactions with the register
type SpockSnapshot struct {
	Snapshot
	SpockSecret []byte
}

func NewView(
	readFunc func(owner string, key string) (flow.RegisterValue, error),
) *View {
	return NewDeltaView(
		state.ReadFuncStorageSnapshot{
			ReadFunc: func(id flow.RegisterID) (flow.RegisterValue, error) {
				return readFunc(id.Owner, id.Key)
			},
		})
}

// NewDeltaView instantiates a new ledger view with the provided read function.
func NewDeltaView(storage state.StorageSnapshot) *View {
	if storage == nil {
		storage = state.EmptyStorageSnapshot{}
	}
	return &View{
		delta:             NewDelta(),
		spockSecretLock:   &sync.Mutex{},
		regTouchSet:       make(map[flow.RegisterID]struct{}),
		storage:           storage,
		spockSecretHasher: hash.NewSHA3_256(),
	}
}

// Snapshot returns copy of current state of interactions with a View
func (v *View) Interactions() *SpockSnapshot {

	var delta = Delta{
		Data: make(map[flow.RegisterID]flow.RegisterValue, len(v.delta.Data)),
	}
	var reads = make(map[flow.RegisterID]struct{}, len(v.regTouchSet))

	bytesWrittenToRegisters := 0
	// copy data
	for s, value := range v.delta.Data {
		delta.Data[s] = value
		bytesWrittenToRegisters += len(value)
	}

	for k := range v.regTouchSet {
		reads[k] = struct{}{}
	}

	return &SpockSnapshot{
		Snapshot: Snapshot{
			Delta: delta,
			Reads: reads,
			SnapshotStats: SnapshotStats{
				NumberOfBytesWrittenToRegisters: bytesWrittenToRegisters,
				NumberOfRegistersTouched:        len(reads),
			},
		},
		SpockSecret: v.SpockSecret(),
	}
}

// AllRegisterIDs returns all the register IDs either in read or delta.
// The returned ids are unsorted.
func (r *Snapshot) AllRegisterIDs() []flow.RegisterID {
	set := make(map[flow.RegisterID]struct{}, len(r.Reads)+len(r.Delta.Data))
	for reg := range r.Reads {
		set[reg] = struct{}{}
	}
	for _, reg := range r.Delta.RegisterIDs() {
		set[reg] = struct{}{}
	}
	ret := make([]flow.RegisterID, 0, len(set))
	for r := range set {
		ret = append(ret, r)
	}
	return ret
}

// NewChild generates a new child view, with the current view as the base, sharing the Get function
func (v *View) NewChild() state.View {
	return NewDeltaView(state.NewPeekerStorageSnapshot(v))
}

func (v *View) Meter() *meter.Meter {
	return nil
}

func (v *View) DropChanges() error {
	v.delta = NewDelta()
	return nil
}

// Get gets a register value from this view.
//
// This function will return an error if it fails to read from the underlying
// data source for this view.
func (v *View) Get(registerID flow.RegisterID) (flow.RegisterValue, error) {
	var err error

	value, exists := v.delta.Get(registerID)
	if !exists {
		value, err = v.storage.Get(registerID)
		if err != nil {
			return nil, fmt.Errorf("get register failed: %w", err)
		}
		// capture register touch
		v.regTouchSet[registerID] = struct{}{}
		// increase reads
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
func (v *View) Peek(id flow.RegisterID) (flow.RegisterValue, error) {
	value, exists := v.delta.Get(id)
	if exists {
		return value, nil
	}

	return v.storage.Get(id)
}

// Set sets a register value in this view.
func (v *View) Set(registerID flow.RegisterID, value flow.RegisterValue) error {
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
	v.regTouchSet[registerID] = struct{}{}
	// add key value to delta
	v.delta.Set(registerID, value)
	return nil
}

// Delta returns a record of the registers that were mutated in this view.
func (v *View) Delta() Delta {
	return v.delta
}

// TODO(patrick): remove after updating emulator
func (view *View) MergeView(child state.View) error {
	return view.Merge(child.Finalize())
}

func (view *View) Finalize() *state.ExecutionSnapshot {
	return &state.ExecutionSnapshot{
		// TODO(patrick): exclude reads that came from the write set
		ReadSet:     view.regTouchSet,
		WriteSet:    view.delta.Data,
		SpockSecret: view.SpockSecret(),
	}
}

func (view *View) Merge(child *state.ExecutionSnapshot) error {
	for id := range child.ReadSet {
		view.regTouchSet[id] = struct{}{}
	}

	_, err := view.spockSecretHasher.Write(child.SpockSecret)
	if err != nil {
		return fmt.Errorf("merging SPoCK secrets failed: %w", err)
	}

	for key, value := range child.WriteSet {
		view.delta.Data[key] = value
	}

	return nil
}

// RegisterTouches returns the register IDs touched by this view (either read or write)
func (r *Snapshot) RegisterTouches() map[flow.RegisterID]struct{} {
	ret := make(map[flow.RegisterID]struct{}, len(r.Reads))
	for k := range r.Reads {
		ret[k] = struct{}{}
	}
	return ret
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
