package snapshot

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestSnapshotTree(t *testing.T) {
	id1 := flow.NewRegisterID(flow.HexToAddress("0x1"), "")
	id2 := flow.NewRegisterID(flow.HexToAddress("0x2"), "")
	id3 := flow.NewRegisterID(flow.HexToAddress("0x3"), "")
	missingId := flow.NewRegisterID(flow.HexToAddress("0x99"), "")

	value1v0 := flow.RegisterValue("1v0")

	// entries:
	// 1 -> 1v0
	tree0 := NewSnapshotTree(
		MapStorageSnapshot{
			id1: value1v0,
		})

	expected0 := map[flow.RegisterID]flow.RegisterValue{
		id1:       value1v0,
		id2:       nil,
		id3:       nil,
		missingId: nil,
	}

	value2v1 := flow.RegisterValue("2v1")

	tree1 := tree0.Append(
		&ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				id2: value2v1,
			},
		})

	expected1 := map[flow.RegisterID]flow.RegisterValue{
		id1:       value1v0,
		id2:       value2v1,
		id3:       nil,
		missingId: nil,
	}

	value1v1 := flow.RegisterValue("1v1")
	value3v1 := flow.RegisterValue("3v1")

	tree2 := tree1.Append(
		&ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				id1: value1v1,
				id3: value3v1,
			},
		})

	expected2 := map[flow.RegisterID]flow.RegisterValue{
		id1:       value1v1,
		id2:       value2v1,
		id3:       value3v1,
		missingId: nil,
	}

	value2v2 := flow.RegisterValue("2v2")

	tree3 := tree2.Append(
		&ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				id2: value2v2,
			},
		})

	expected3 := map[flow.RegisterID]flow.RegisterValue{
		id1:       value1v1,
		id2:       value2v2,
		id3:       value3v1,
		missingId: nil,
	}

	expectedCompacted := map[flow.RegisterID]flow.RegisterValue{
		id1:       value1v1,
		id2:       value2v2,
		id3:       value3v1,
		missingId: nil,
	}

	compactedTree := tree3
	numExtraUpdates := 2*compactThreshold + 1
	for i := 0; i < numExtraUpdates; i++ {
		value := []byte(fmt.Sprintf("compacted %d", i))
		expectedCompacted[id3] = value
		compactedTree = compactedTree.Append(
			&ExecutionSnapshot{
				WriteSet: map[flow.RegisterID]flow.RegisterValue{
					id3: value,
				},
			})
	}

	check := func(
		tree SnapshotTree,
		expected map[flow.RegisterID]flow.RegisterValue,
		compactedLogLen int,
	) {
		require.Len(t, tree.compactedLog, compactedLogLen)

		for key, expectedValue := range expected {
			value, err := tree.Get(key)
			require.NoError(t, err)
			require.Equal(t, value, expectedValue, string(expectedValue))
		}
	}

	check(tree0, expected0, 0)
	check(tree1, expected1, 1)
	check(tree2, expected2, 2)
	check(tree3, expected3, 3)
	check(compactedTree, expectedCompacted, 4)

	emptyTree := NewSnapshotTree(nil)
	value, err := emptyTree.Get(id1)
	require.NoError(t, err)
	require.Nil(t, value)
}
