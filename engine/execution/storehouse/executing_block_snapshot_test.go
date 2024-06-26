package storehouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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
		require.Equal(t, []byte(nil), value)
	})

	t.Run("Extend snapshot", func(t *testing.T) {
		reg1 := makeReg("key1", "val1")
		reg2 := makeReg("key2", "val2")
		base := snapshot.MapStorageSnapshot{
			reg1.Key: reg1.Value,
			reg2.Key: reg2.Value,
		}
		// snap1: { key1: val1, key2: val2 }
		snap1 := storehouse.NewExecutingBlockSnapshot(base, unittest.StateCommitmentFixture())

		updatedReg2 := makeReg("key2", "val22")
		reg3 := makeReg("key3", "val3")
		// snap2: { key1: val1, key2: val22, key3: val3 }
		snap2 := snap1.Extend(unittest.StateCommitmentFixture(), map[flow.RegisterID]flow.RegisterValue{
			updatedReg2.Key: updatedReg2.Value,
			reg3.Key:        reg3.Value,
		})

		// should get un-changed value
		value, err := snap2.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, []byte("val1"), value)

		value, err = snap2.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, []byte("val22"), value)

		value, err = snap2.Get(reg3.Key)
		require.NoError(t, err)
		require.Equal(t, []byte("val3"), value)

		// should get nil for unknown register
		unknown := makeReg("unknown", "unknownV")
		value, err = snap2.Get(unknown.Key)
		require.NoError(t, err)
		require.Equal(t, []byte(nil), value)

		// create snap3 with reg3 updated
		// snap3: { key1: val1, key2: val22, key3: val33 }
		updatedReg3 := makeReg("key3", "val33")
		snap3 := snap2.Extend(unittest.StateCommitmentFixture(), map[flow.RegisterID]flow.RegisterValue{
			updatedReg3.Key: updatedReg3.Value,
		})

		// verify all keys
		value, err = snap3.Get(reg1.Key)
		require.NoError(t, err)
		require.Equal(t, []byte("val1"), value)

		value, err = snap3.Get(reg2.Key)
		require.NoError(t, err)
		require.Equal(t, []byte("val22"), value)

		value, err = snap3.Get(reg3.Key)
		require.NoError(t, err)
		require.Equal(t, []byte("val33"), value)
	})
}
