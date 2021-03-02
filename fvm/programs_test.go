package fvm

import (
	"testing"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/handler"
)

func Test_Programs(t *testing.T) {

	someProgram := &interpreter.Program{
		Program:     &ast.Program{},
		Elaboration: nil,
	}
	someLocation := common.IdentifierLocation("some")

	addressLocation := common.AddressLocation{
		Address: common.BytesToAddress([]byte{2, 3, 4}),
		Name:    "address",
	}

	t.Run("cleanup without changed programs", func(t *testing.T) {

		parentLocation := common.IdentifierLocation("parent")

		parent := NewEmptyPrograms()
		parent.Set(parentLocation, &interpreter.Program{})

		programs := parent.ChildPrograms()
		programs.Set(someLocation, someProgram)
		programs.Set(addressLocation, &interpreter.Program{})

		retrieved := programs.Get(someLocation)
		require.NotNil(t, retrieved)
		retrieved = programs.Get(addressLocation)
		require.NotNil(t, retrieved)
		retrieved = programs.Get(parentLocation)
		require.NotNil(t, retrieved)

		programs.Cleanup(nil)

		retrieved = programs.Get(someLocation)
		require.Nil(t, retrieved)

		retrieved = programs.Get(addressLocation)
		require.NotNil(t, retrieved)

		retrieved = programs.Get(parentLocation)
		require.NotNil(t, retrieved)
	})

	t.Run("cleanup with changed programs", func(t *testing.T) {

		parentLocation := common.IdentifierLocation("parent")

		parent := NewEmptyPrograms()
		parent.Set(parentLocation, &interpreter.Program{})

		programs := parent.ChildPrograms()
		programs.Set(someLocation, someProgram)
		programs.Set(addressLocation, &interpreter.Program{})

		retrieved := programs.Get(someLocation)
		require.NotNil(t, retrieved)
		retrieved = programs.Get(addressLocation)
		require.NotNil(t, retrieved)
		retrieved = programs.Get(parentLocation)
		require.NotNil(t, retrieved)

		// we don't care about the changed program, just their amount (for now)
		programs.Cleanup([]handler.ContractUpdateKey{{}, {}})

		retrieved = programs.Get(someLocation)
		require.Nil(t, retrieved)
		retrieved = programs.Get(addressLocation)
		require.Nil(t, retrieved)
		retrieved = programs.Get(parentLocation)
		require.Nil(t, retrieved)
	})

	t.Run("forking", func(t *testing.T) {
		parent := NewEmptyPrograms()
		parent.Set(someLocation, someProgram)

		childA := parent.ChildPrograms()
		childB := parent.ChildPrograms()

		// Both child have item
		retrieved := childA.Get(someLocation)
		require.NotNil(t, retrieved)
		retrieved = childB.Get(someLocation)
		require.NotNil(t, retrieved)

		// changed in child don't influence other forks
		childA.Set(someLocation, nil)

		retrieved = childA.Get(someLocation)
		require.Nil(t, retrieved)
		retrieved = childB.Get(someLocation)
		require.NotNil(t, retrieved)

		childB.Set(addressLocation, &interpreter.Program{})

		retrieved = childA.Get(addressLocation)
		require.Nil(t, retrieved)
		retrieved = childB.Get(addressLocation)
		require.NotNil(t, retrieved)
	})

	t.Run("changes", func(t *testing.T) {
		parent := NewEmptyPrograms()
		require.False(t, parent.HasChanges())

		parent.Set(someLocation, someProgram)
		require.True(t, parent.HasChanges())

		child := parent.ChildPrograms()
		require.False(t, child.HasChanges())

		child.Cleanup([]handler.ContractUpdateKey{{}, {}})
		require.True(t, child.HasChanges())

		child = parent.ChildPrograms()

		// getting values doesn't count as change
		retrieved := child.Get(someLocation)
		require.NotNil(t, retrieved)
		require.False(t, child.HasChanges())

		child.Set(someLocation, someProgram)

		require.True(t, child.HasChanges())
	})

}
