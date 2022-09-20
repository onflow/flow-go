package programs

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestChainPrograms(t *testing.T) {
	testBlockId := func(i byte) flow.Identifier {
		id := flow.ZeroID
		id[0] = i
		return id
	}

	testLocation := func(hex string) common.AddressLocation {
		return common.AddressLocation{
			Address: common.Address(flow.HexToAddress(hex)),
			Name:    hex,
		}
	}

	programs, err := NewChainPrograms(2)
	require.NoError(t, err)

	//
	// Creating a BlockPrograms from scratch
	//

	blockId1 := testBlockId(1)

	block1 := programs.GetOrCreateBlockPrograms(blockId1, flow.ZeroID)
	require.NotNil(t, block1)

	loc1 := testLocation("0a")
	prog1 := &interpreter.Program{}

	block1.Set(loc1, prog1, nil)
	block1.Cleanup(ModifiedSetsInvalidator{}) // aka commit

	foundProg, _, ok := block1.GetForTestingOnly(loc1)
	require.True(t, ok)
	require.Same(t, prog1, foundProg)

	//
	// Creating a BlockPrograms from parent
	//

	blockId2 := testBlockId(2)

	block2 := programs.GetOrCreateBlockPrograms(blockId2, blockId1)
	require.NotNil(t, block2)
	require.NotSame(t, block1, block2)

	loc2 := testLocation("0b")
	prog2 := &interpreter.Program{}

	block2.Set(loc2, prog2, nil)
	block2.Cleanup(ModifiedSetsInvalidator{}) // aka commit

	foundProg, _, ok = block2.GetForTestingOnly(loc1)
	require.True(t, ok)
	require.Same(t, prog1, foundProg)

	foundProg, _, ok = block2.GetForTestingOnly(loc2)
	require.True(t, ok)
	require.Same(t, prog2, foundProg)

	//
	// Reuse exising BlockPrograms in cache
	//

	foundBlock := programs.GetOrCreateBlockPrograms(blockId1, flow.ZeroID)
	require.Same(t, block1, foundBlock)

	foundBlock = programs.Get(blockId1)
	require.Same(t, block1, foundBlock)

	foundProg, _, ok = block1.GetForTestingOnly(loc1)
	require.True(t, ok)
	require.Same(t, prog1, foundProg)

	// writes to block2 did't poplute block1.
	_, _, ok = block1.GetForTestingOnly(loc2)
	require.False(t, ok)

	//
	// Test eviction
	//

	blockId3 := testBlockId(3)

	// block3 forces block1 to evict
	block3 := programs.GetOrCreateBlockPrograms(blockId3, blockId2)
	require.NotNil(t, block3)
	require.NotSame(t, block1, block3)
	require.NotSame(t, block2, block3)

	foundProg, _, ok = block3.GetForTestingOnly(loc1)
	require.True(t, ok)
	require.Same(t, prog1, foundProg)

	foundProg, _, ok = block3.GetForTestingOnly(loc2)
	require.True(t, ok)
	require.Same(t, prog2, foundProg)

	// block1 forces block2 to evict
	foundBlock = programs.GetOrCreateBlockPrograms(blockId1, flow.ZeroID)
	require.NotSame(t, block1, foundBlock)

	_, _, ok = foundBlock.GetForTestingOnly(loc1)
	require.False(t, ok)

	_, _, ok = foundBlock.GetForTestingOnly(loc2)
	require.False(t, ok)

	block1 = foundBlock

	//
	// Scripts don't poplute the cache
	//

	// Create from cached current block
	scriptBlock := programs.NewBlockProgramsForScript(blockId3)
	require.NotNil(t, scriptBlock)
	require.NotSame(t, block2, scriptBlock)
	require.NotSame(t, block3, scriptBlock)

	foundProg, _, ok = scriptBlock.GetForTestingOnly(loc1)
	require.True(t, ok)
	require.Same(t, prog1, foundProg)

	foundProg, _, ok = scriptBlock.GetForTestingOnly(loc2)
	require.True(t, ok)
	require.Same(t, prog2, foundProg)

	foundBlock = programs.Get(blockId3)
	require.Same(t, block3, foundBlock)

	foundBlock = programs.Get(blockId1)
	require.Same(t, block1, foundBlock)

	foundBlock = programs.Get(blockId2)
	require.Nil(t, foundBlock)
}
