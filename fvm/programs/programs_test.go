package programs

import (
	"testing"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func Test_Programs(t *testing.T) {

	someProgram := &interpreter.Program{
		Program:     &ast.Program{},
		Elaboration: nil,
	}
	someLocation := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{6, 6, 6}),
		Name:    "blah",
	}

	newState := state.NewState(
		utils.NewSimpleView(),
		meter.NewMeter(meter.DefaultParameters()),
		state.DefaultParameters(),
	)

	addressLocation := common.AddressLocation{
		Address: common.MustBytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	t.Run("cleanup without changed programs", func(t *testing.T) {
		parentLocation := common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{3, 4, 5}),
			Name:    "parent",
		}

		parent := NewEmptyPrograms()

		parent.Set(parentLocation, &interpreter.Program{}, newState)

		programs := parent.ChildPrograms()
		programs.Set(someLocation, someProgram, newState)
		programs.Set(addressLocation, &interpreter.Program{}, newState)

		retrieved, _, has := programs.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(addressLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(parentLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		programs.Cleanup(ModifiedSets{})

		retrieved, _, has = programs.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(addressLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(parentLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)
	})

	t.Run("cleanup with changed programs", func(t *testing.T) {
		parentLocation := common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{3, 4, 5}),
			Name:    "parent",
		}

		parent := NewEmptyPrograms()
		parent.Set(parentLocation, &interpreter.Program{}, newState)

		programs := parent.ChildPrograms()
		programs.Set(someLocation, someProgram, newState)
		programs.Set(addressLocation, &interpreter.Program{}, newState)

		retrieved, _, has := programs.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(addressLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(parentLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		programs.Cleanup(ModifiedSets{
			// we don't care about the changed program, just their amount (for
			// now)
			[]ContractUpdateKey{{}, {}},
			nil,
		})

		retrieved, _, has = programs.Get(someLocation)
		require.Nil(t, retrieved)
		require.False(t, has)

		retrieved, _, has = programs.Get(addressLocation)
		require.Nil(t, retrieved)
		require.False(t, has)

		retrieved, _, has = programs.Get(parentLocation)
		require.Nil(t, retrieved)
		require.False(t, has)
	})

	t.Run("cleanup with frozen accounts", func(t *testing.T) {

		parentLocation := common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{3, 4, 5}),
			Name:    "parent",
		}

		parent := NewEmptyPrograms()
		parent.Set(parentLocation, &interpreter.Program{}, newState)

		programs := parent.ChildPrograms()
		programs.Set(someLocation, someProgram, newState)
		programs.Set(addressLocation, &interpreter.Program{}, newState)

		retrieved, _, has := programs.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(addressLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = programs.Get(parentLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		programs.Cleanup(ModifiedSets{
			nil,
			// For now, we don't care about the frozen account's address, just
			// their presences.
			make([]common.Address, 1),
		})

		retrieved, _, has = programs.Get(someLocation)
		require.Nil(t, retrieved)
		require.False(t, has)

		retrieved, _, has = programs.Get(addressLocation)
		require.Nil(t, retrieved)
		require.False(t, has)

		retrieved, _, has = programs.Get(parentLocation)
		require.Nil(t, retrieved)
		require.False(t, has)
	})

	t.Run("forking", func(t *testing.T) {
		parent := NewEmptyPrograms()
		parent.Set(someLocation, someProgram, newState)

		childA := parent.ChildPrograms()
		childB := parent.ChildPrograms()

		// Both child have item
		retrieved, _, has := childA.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = childB.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		// changed in child don't influence other forks
		childA.Set(someLocation, nil, newState)

		retrieved, _, has = childA.Get(someLocation)
		require.Nil(t, retrieved)
		require.True(t, has)

		retrieved, _, has = childB.Get(someLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)

		childB.Set(addressLocation, &interpreter.Program{}, newState)

		retrieved, _, has = childA.Get(addressLocation)
		require.Nil(t, retrieved)
		require.False(t, has)

		retrieved, _, has = childB.Get(addressLocation)
		require.NotNil(t, retrieved)
		require.True(t, has)
	})

	t.Run("changes", func(t *testing.T) {
		parent := NewEmptyPrograms()
		require.False(t, parent.HasChanges())

		parent.Set(someLocation, someProgram, newState)
		require.True(t, parent.HasChanges())

		child := parent.ChildPrograms()
		require.False(t, child.HasChanges())

		child.Cleanup(ModifiedSets{
			[]ContractUpdateKey{{}, {}},
			nil,
		})
		require.True(t, child.HasChanges())

		child = parent.ChildPrograms()

		// getting values doesn't count as change
		retrieved, _, has := child.Get(someLocation)
		require.NotNil(t, retrieved)
		require.False(t, child.HasChanges())
		require.True(t, has)

		child.Set(someLocation, someProgram, newState)

		require.True(t, child.HasChanges())
	})

}
