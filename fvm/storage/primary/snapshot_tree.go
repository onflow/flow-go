package primary

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
)

type timestampedSnapshotTree struct {
	currentSnapshotTime logical.Time
	baseSnapshotTime    logical.Time

	snapshot.SnapshotTree

	fullLog snapshot.UpdateLog
}

func newTimestampedSnapshotTree(
	storageSnapshot snapshot.StorageSnapshot,
	snapshotTime logical.Time,
) timestampedSnapshotTree {
	return timestampedSnapshotTree{
		currentSnapshotTime: snapshotTime,
		baseSnapshotTime:    snapshotTime,
		SnapshotTree:        snapshot.NewSnapshotTree(storageSnapshot),
		fullLog:             nil,
	}
}

func (tree timestampedSnapshotTree) Append(
	executionSnapshot *snapshot.ExecutionSnapshot,
) timestampedSnapshotTree {
	return timestampedSnapshotTree{
		currentSnapshotTime: tree.currentSnapshotTime + 1,
		baseSnapshotTime:    tree.baseSnapshotTime,
		SnapshotTree:        tree.SnapshotTree.Append(executionSnapshot),
		fullLog:             append(tree.fullLog, executionSnapshot.WriteSet),
	}
}

func (tree timestampedSnapshotTree) SnapshotTime() logical.Time {
	return tree.currentSnapshotTime
}

func (tree timestampedSnapshotTree) UpdatesSince(
	snapshotTime logical.Time,
) (
	snapshot.UpdateLog,
	error,
) {
	if snapshotTime < tree.baseSnapshotTime {
		// This should never happen.
		return nil, fmt.Errorf(
			"missing update log range [%v, %v)",
			snapshotTime,
			tree.baseSnapshotTime)
	}

	if snapshotTime > tree.currentSnapshotTime {
		// This should never happen.
		return nil, fmt.Errorf(
			"missing update log range (%v, %v]",
			tree.currentSnapshotTime,
			snapshotTime)
	}

	return tree.fullLog[int(snapshotTime-tree.baseSnapshotTime):], nil
}

type rebaseableTimestampedSnapshotTree struct {
	timestampedSnapshotTree
}

func newRebaseableTimestampedSnapshotTree(
	snapshotTree timestampedSnapshotTree,
) *rebaseableTimestampedSnapshotTree {
	return &rebaseableTimestampedSnapshotTree{
		timestampedSnapshotTree: snapshotTree,
	}
}

func (tree *rebaseableTimestampedSnapshotTree) Rebase(
	base timestampedSnapshotTree,
) {
	tree.timestampedSnapshotTree = base
}
