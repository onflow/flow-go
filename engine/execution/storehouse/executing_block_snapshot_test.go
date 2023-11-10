package storehouse_test

import (
	"testing"

	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestExtendingBlockSnapshot(t *testing.T) {
	t.Run("Get register", func(t *testing.T) {
		reg1 := makeReg("key1", "val1")
		base := snapshot.MapStorageSnapshot{
			reg1.Key: reg1.Value,
		}
		baseCommit := unittest.StateCommitmentFixture()
		snap := storehouse.NewExecutingBlockSnapshot(base, baseCommit)

		// should get value
		value, err := snap.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, reg1.Value, value)

		// should get nil for unknown register
		unknown := makeReg("unknown", "unknownV")
		value, err = snap.Get(unknown.Key)
		require.NoError(t, err)
		require.Equal(t, nil, value)
	})

	t.Run("Extend snapshot", func(t *testing.T) {
		reg1 := makeReg("key1", "val1")
		reg2 := makeReg("key2", "val2")
		base := snapshot.MapStorageSnapshot{
			reg1.Key: reg1.Value,
			reg2.Key: reg2.Value,
		}
		snap1Commit := unittest.StateCommitmentFixture()
		snap1 := storehouse.NewExecutingBlockSnapshot(base, snap1Commit)

		updatedReg2 := makeReg("key2", "val22")
		reg3 := makeReg("key3", "val3")
		snap2Commit := unittest.StateCommitmentFixture()
		snap2 := snap1.Extend(snap2Commit, flow.RegisterEntries{
			updatedReg2, reg3,
		})

		// should get un-changed value
		value, err := snap2.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, reg1.Value, value)

		// should get nil for unknown register
		unknown := makeReg("unknown", "unknownV")
		value, err = snap2.Get(unknown.Key)
		require.NoError(t, err)
		require.Equal(t, nil, value)

		// should get updated value
		value, err = snap2.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, updatedReg2.Value, value)
	})
}

func makeReg(key string, value string) flow.RegisterEntry {
	panic("")
}
