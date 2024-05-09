package primary

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

func TestTimestampedSnapshotTree(t *testing.T) {
	// Test setup ("commit" 4 execution snapshots to the base tree)

	baseSnapshotTime := logical.Time(5)

	registerId0 := flow.RegisterID{
		Owner: "",
		Key:   "key0",
	}
	value0 := flow.RegisterValue([]byte("value0"))

	tree0 := newTimestampedSnapshotTree(
		snapshot.MapStorageSnapshot{
			registerId0: value0,
		},
		baseSnapshotTime)

	registerId1 := flow.RegisterID{
		Owner: "",
		Key:   "key1",
	}
	value1 := flow.RegisterValue([]byte("value1"))
	writeSet1 := map[flow.RegisterID]flow.RegisterValue{
		registerId1: value1,
	}

	tree1 := tree0.Append(
		&snapshot.ExecutionSnapshot{
			WriteSet: writeSet1,
		})

	registerId2 := flow.RegisterID{
		Owner: "",
		Key:   "key2",
	}
	value2 := flow.RegisterValue([]byte("value2"))
	writeSet2 := map[flow.RegisterID]flow.RegisterValue{
		registerId2: value2,
	}

	tree2 := tree1.Append(
		&snapshot.ExecutionSnapshot{
			WriteSet: writeSet2,
		})

	registerId3 := flow.RegisterID{
		Owner: "",
		Key:   "key3",
	}
	value3 := flow.RegisterValue([]byte("value3"))
	writeSet3 := map[flow.RegisterID]flow.RegisterValue{
		registerId3: value3,
	}

	tree3 := tree2.Append(
		&snapshot.ExecutionSnapshot{
			WriteSet: writeSet3,
		})

	registerId4 := flow.RegisterID{
		Owner: "",
		Key:   "key4",
	}
	value4 := flow.RegisterValue([]byte("value4"))
	writeSet4 := map[flow.RegisterID]flow.RegisterValue{
		registerId4: value4,
	}

	tree4 := tree3.Append(
		&snapshot.ExecutionSnapshot{
			WriteSet: writeSet4,
		})

	// Verify the trees internal values

	trees := []timestampedSnapshotTree{tree0, tree1, tree2, tree3, tree4}
	logs := snapshot.UpdateLog{writeSet1, writeSet2, writeSet3, writeSet4}

	for i, tree := range trees {
		require.Equal(t, baseSnapshotTime, tree.baseSnapshotTime)
		require.Equal(
			t,
			baseSnapshotTime+logical.Time(i),
			tree.SnapshotTime())
		if i == 0 {
			require.Nil(t, tree.fullLog)
		} else {
			require.Equal(t, logs[:i], tree.fullLog)
		}

		value, err := tree.Get(registerId0)
		require.NoError(t, err)
		require.Equal(t, value0, value)

		value, err = tree.Get(registerId1)
		require.NoError(t, err)
		if i >= 1 {
			require.Equal(t, value1, value)
		} else {
			require.Nil(t, value)
		}

		value, err = tree.Get(registerId2)
		require.NoError(t, err)
		if i >= 2 {
			require.Equal(t, value2, value)
		} else {
			require.Nil(t, value)
		}

		value, err = tree.Get(registerId3)
		require.NoError(t, err)
		if i >= 3 {
			require.Equal(t, value3, value)
		} else {
			require.Nil(t, value)
		}

		value, err = tree.Get(registerId4)
		require.NoError(t, err)
		if i == 4 {
			require.Equal(t, value4, value)
		} else {
			require.Nil(t, value)
		}
	}

	// Verify UpdatesSince returns

	updates, err := tree0.UpdatesSince(baseSnapshotTime)
	require.NoError(t, err)
	require.Nil(t, updates)

	_, err = tree4.UpdatesSince(baseSnapshotTime - 1)
	require.ErrorContains(t, err, "missing update log range [4, 5)")

	for i := 0; i < 5; i++ {
		updates, err = tree4.UpdatesSince(baseSnapshotTime + logical.Time(i))
		require.NoError(t, err)
		require.Equal(t, logs[i:], updates)
	}

	snapshotTime := baseSnapshotTime + logical.Time(5)
	require.Equal(t, tree4.SnapshotTime()+1, snapshotTime)

	_, err = tree4.UpdatesSince(snapshotTime)
	require.ErrorContains(t, err, "missing update log range (9, 10]")
}

func TestRebaseableTimestampedSnapshotTree(t *testing.T) {
	registerId := flow.RegisterID{
		Owner: "owner",
		Key:   "key",
	}

	value1 := flow.RegisterValue([]byte("value1"))
	value2 := flow.RegisterValue([]byte("value2"))

	tree1 := newTimestampedSnapshotTree(
		snapshot.MapStorageSnapshot{
			registerId: value1,
		},
		0)

	tree2 := newTimestampedSnapshotTree(
		snapshot.MapStorageSnapshot{
			registerId: value2,
		},
		0)

	rebaseableTree := newRebaseableTimestampedSnapshotTree(tree1)
	treeReference := rebaseableTree

	value, err := treeReference.Get(registerId)
	require.NoError(t, err)
	require.Equal(t, value, value1)

	rebaseableTree.Rebase(tree2)

	value, err = treeReference.Get(registerId)
	require.NoError(t, err)
	require.Equal(t, value, value2)
}
