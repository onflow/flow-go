package utils

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(patrick): combine this with storage.testutils.TestStorageSnapshot
// once #3962 is merged.
type MapStorageSnapshot map[flow.RegisterID]flow.RegisterValue

func (storage MapStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return storage[id], nil
}

// NewStorageSnapshotFromPayload returns an instance of StorageSnapshot with
// entries loaded from payloads (should only be used for migration)
func NewStorageSnapshotFromPayload(
	payloads []ledger.Payload,
) MapStorageSnapshot {
	snapshot := make(MapStorageSnapshot, len(payloads))
	for _, entry := range payloads {
		key, err := entry.Key()
		if err != nil {
			panic(err)
		}

		id := flow.NewRegisterID(
			string(key.KeyParts[0].Value),
			string(key.KeyParts[1].Value))

		snapshot[id] = entry.Value()
	}

	return snapshot
}

// TODO(patrick): rename to MigrationView
// SimpleView provides a simple view for testing and migration purposes.
type SimpleView struct {
	// Get/Set/DropDelta are guarded by mutex since migration concurrently
	// assess the same view.
	//
	// Note that we can't use RWLock since all view access, including Get,
	// mutate the view's internal state.
	sync.Mutex
	base state.View
}

func NewSimpleView() *SimpleView {
	return &SimpleView{
		base: delta.NewDeltaView(nil),
	}
}

func NewSimpleViewFromPayloads(payloads []ledger.Payload) *SimpleView {
	return &SimpleView{
		base: delta.NewDeltaView(NewStorageSnapshotFromPayload(payloads)),
	}
}

func (view *SimpleView) NewChild() state.View {
	return &SimpleView{
		base: view.base.NewChild(),
	}
}

func (view *SimpleView) MergeView(o state.View) error {
	other, ok := o.(*SimpleView)
	if !ok {
		return fmt.Errorf("can not merge: view type mismatch (given: %T, expected:SimpleView)", o)
	}

	return view.base.MergeView(other.base)
}

func (view *SimpleView) DropDelta() {
	view.Lock()
	defer view.Unlock()

	view.base.DropDelta()
}

func (view *SimpleView) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	view.Lock()
	defer view.Unlock()

	return view.base.Get(id)
}

func (view *SimpleView) Set(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	view.Lock()
	defer view.Unlock()

	return view.base.Set(id, value)
}

func (view *SimpleView) AllRegisterIDs() []flow.RegisterID {
	return view.base.AllRegisterIDs()
}

func (view *SimpleView) UpdatedRegisterIDs() []flow.RegisterID {
	return view.base.UpdatedRegisterIDs()
}

func (view *SimpleView) UpdatedRegisters() flow.RegisterEntries {
	return view.base.UpdatedRegisters()
}

func (view *SimpleView) UpdatedPayloads() []ledger.Payload {
	updates := view.UpdatedRegisters()

	ret := make([]ledger.Payload, 0, len(updates))
	for _, entry := range updates {
		key := registerIdToLedgerKey(entry.Key)
		ret = append(ret, *ledger.NewPayload(key, ledger.Value(entry.Value)))
	}

	return ret
}

func registerIdToLedgerKey(id flow.RegisterID) ledger.Key {
	keyParts := []ledger.KeyPart{
		ledger.NewKeyPart(0, []byte(id.Owner)),
		ledger.NewKeyPart(2, []byte(id.Key)),
	}

	return ledger.NewKey(keyParts)
}
