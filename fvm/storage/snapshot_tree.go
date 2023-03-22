package storage

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

const (
	compactThreshold = 10
)

type updateLog []map[flow.RegisterID]flow.RegisterValue

// SnapshotTree is a simple LSM tree representation of the key/value storage
// at a given point in time.
type SnapshotTree struct {
	base state.StorageSnapshot

	fullLog      updateLog
	compactedLog updateLog
}

// NewSnapshotTree returns a tree with keys/values initialized to the base
// storage snapshot.
func NewSnapshotTree(base state.StorageSnapshot) SnapshotTree {
	return SnapshotTree{
		base:         base,
		fullLog:      nil,
		compactedLog: nil,
	}
}

// Append returns a new tree with updates from the execution snapshot "applied"
// to the original original tree.
func (tree SnapshotTree) Append(
	update *state.ExecutionSnapshot,
) SnapshotTree {
	compactedLog := tree.compactedLog
	if len(update.WriteSet) > 0 {
		compactedLog = append(tree.compactedLog, update.WriteSet)
		if len(compactedLog) > compactThreshold {
			size := 0
			for _, set := range compactedLog {
				size += len(set)
			}

			mergedSet := make(map[flow.RegisterID]flow.RegisterValue, size)
			for _, set := range compactedLog {
				for id, value := range set {
					mergedSet[id] = value
				}
			}

			compactedLog = updateLog{mergedSet}
		}
	}

	return SnapshotTree{
		base:         tree.base,
		fullLog:      append(tree.fullLog, update.WriteSet),
		compactedLog: compactedLog,
	}
}

// Get returns the register id's value.
func (tree SnapshotTree) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	for idx := len(tree.compactedLog) - 1; idx >= 0; idx-- {
		value, ok := tree.compactedLog[idx][id]
		if ok {
			return value, nil
		}
	}

	if tree.base != nil {
		return tree.base.Get(id)
	}

	return nil, nil
}
