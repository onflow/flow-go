package storage

import (
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"

	"github.com/google/btree"
)

type UpdateLog []map[flow.RegisterID]flow.RegisterValue

// SnapshotTree is a simple LSM tree representation of the key/value storage
// at a given point in time.
type SnapshotTree struct {
	scratch *btree.BTreeG[flow.RegisterEntry]
	base    state.StorageSnapshot
}

// NewSnapshotTree returns a tree with keys/values initialized to the base
// storage snapshot.
func NewSnapshotTree(base state.StorageSnapshot) SnapshotTree {
	return SnapshotTree{
		scratch: btree.NewG(
			8,
			func(a, b flow.RegisterEntry) bool {
				if a.Key.Owner != b.Key.Owner {
					return a.Key.Owner < b.Key.Owner
				}
				return a.Key.Key < b.Key.Key
			},
		),
		base: base,
	}
}

// Append returns a new tree with updates from the execution snapshot "applied"
// to the original original tree.
func (tree SnapshotTree) Append(
	update *state.ExecutionSnapshot,
) SnapshotTree {
	if len(update.WriteSet) == 0 {
		return tree
	}

	scratch := tree.scratch.Clone()
	for key, value := range update.WriteSet {
		_, _ = scratch.ReplaceOrInsert(
			flow.RegisterEntry{
				Key:   key,
				Value: value,
			},
		)
	}

	return SnapshotTree{
		scratch: scratch,
		base:    tree.base,
	}
}

// Get returns the register id's value.
func (tree SnapshotTree) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	entity, ok := tree.scratch.Get(flow.RegisterEntry{Key: id})
	if ok {
		return entity.Value, nil
	}

	if tree.base != nil {
		return tree.base.Get(id)
	}

	return nil, nil
}
