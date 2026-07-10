package pebble

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRollbackRegisterStoreToHeight verifies that rolling back removes every register
// update above the target height, lowers the latest height, and leaves the state at the
// target height unchanged.
func TestRollbackRegisterStoreToHeight(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		db := NewBootstrappedRegistersWithPathForTest(t, dir, 0, 0)
		defer func() { require.NoError(t, db.Close()) }()

		r, err := NewRegisters(db, PruningDisabled)
		require.NoError(t, err)

		regA := flow.RegisterID{Owner: "o", Key: "A"}
		regB := flow.RegisterID{Owner: "o", Key: "B"}
		regC := flow.RegisterID{Owner: "o", Key: "C"}

		store := func(h uint64, entries flow.RegisterEntries) {
			require.NoError(t, r.Store(entries, h))
		}
		// regA updated at 1,3,5; regB at 2,4; regC only at 4 (first appears above target)
		store(1, flow.RegisterEntries{{Key: regA, Value: []byte("a1")}})
		store(2, flow.RegisterEntries{{Key: regB, Value: []byte("b2")}})
		store(3, flow.RegisterEntries{{Key: regA, Value: []byte("a3")}})
		store(4, flow.RegisterEntries{{Key: regB, Value: []byte("b4")}, {Key: regC, Value: []byte("c4")}})
		store(5, flow.RegisterEntries{{Key: regA, Value: []byte("a5")}})

		updates := map[uint64][]flow.RegisterID{
			1: {regA},
			2: {regB},
			3: {regA},
			4: {regB, regC},
			5: {regA},
		}
		provider := func(h uint64) ([]flow.RegisterID, error) {
			regs, ok := updates[h]
			if !ok {
				return nil, fmt.Errorf("no updates for height %d", h)
			}
			return regs, nil
		}

		// roll back to height 3: removes A@5, B@4, C@4
		err = RollbackRegisterStoreToHeight(unittest.Logger(), db, 3, provider)
		require.NoError(t, err)

		after, err := NewRegisters(db, PruningDisabled)
		require.NoError(t, err)
		require.Equal(t, uint64(3), after.LatestHeight())

		// values at the target height are preserved
		v, err := after.Get(regA, 3)
		require.NoError(t, err)
		require.Equal(t, []byte("a3"), v)

		v, err = after.Get(regB, 3)
		require.NoError(t, err)
		require.Equal(t, []byte("b2"), v) // b4@4 removed, falls back to b2@2

		_, err = after.Get(regC, 3)
		require.ErrorIs(t, err, storage.ErrNotFound) // c4@4 removed, no value at or below 3

		// heights above the target are no longer indexed
		_, err = after.Get(regA, 4)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
	})
}

// TestRollbackRegisterStoreToHeight_Preconditions verifies the no-op and rejection cases.
func TestRollbackRegisterStoreToHeight_Preconditions(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		db := NewBootstrappedRegistersWithPathForTest(t, dir, 2, 2)
		defer func() { require.NoError(t, db.Close()) }()

		r, err := NewRegisters(db, PruningDisabled)
		require.NoError(t, err)

		reg := flow.RegisterID{Owner: "o", Key: "k"}
		require.NoError(t, r.Store(flow.RegisterEntries{{Key: reg, Value: []byte("v3")}}, 3))
		require.NoError(t, r.Store(flow.RegisterEntries{{Key: reg, Value: []byte("v4")}}, 4))

		provider := func(h uint64) ([]flow.RegisterID, error) {
			return []flow.RegisterID{reg}, nil
		}

		// target above latest: rejected, no writes
		err = RollbackRegisterStoreToHeight(unittest.Logger(), db, 5, provider)
		require.Error(t, err)

		// target below first indexed height: rejected
		err = RollbackRegisterStoreToHeight(unittest.Logger(), db, 1, provider)
		require.Error(t, err)

		// target equal to latest: no-op, latest unchanged
		err = RollbackRegisterStoreToHeight(unittest.Logger(), db, 4, provider)
		require.NoError(t, err)
		after, err := NewRegisters(db, PruningDisabled)
		require.NoError(t, err)
		require.Equal(t, uint64(4), after.LatestHeight())
	})
}
