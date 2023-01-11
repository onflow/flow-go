package derived

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestDerivedChainData(t *testing.T) {
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

	programs, err := NewDerivedChainData(2)
	require.NoError(t, err)

	//
	// Creating a DerivedBlockData from scratch
	//

	blockId1 := testBlockId(1)

	block1 := programs.GetOrCreateDerivedBlockData(blockId1, flow.ZeroID)
	require.NotNil(t, block1)

	loc1 := testLocation("0a")
	prog1 := &Program{
		Program: &interpreter.Program{},
	}

	txn, err := block1.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	txn.SetProgram(loc1, prog1, nil)
	err = txn.Commit()
	require.NoError(t, err)

	entry := block1.programs.GetForTestingOnly(loc1)
	require.NotNil(t, entry)
	require.Same(t, prog1, entry.Value)

	//
	// Creating a DerivedBlockData from parent
	//

	blockId2 := testBlockId(2)

	block2 := programs.GetOrCreateDerivedBlockData(blockId2, blockId1)
	require.NotNil(t, block2)
	require.NotSame(t, block1, block2)

	loc2 := testLocation("0b")
	prog2 := &Program{
		Program: &interpreter.Program{},
	}

	txn, err = block2.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	txn.SetProgram(loc2, prog2, nil)
	err = txn.Commit()
	require.NoError(t, err)

	entry = block2.programs.GetForTestingOnly(loc1)
	require.NotNil(t, entry)
	require.Same(t, prog1, entry.Value)

	entry = block2.programs.GetForTestingOnly(loc2)
	require.NotNil(t, entry)
	require.Same(t, prog2, entry.Value)

	//
	// Reuse exising DerivedBlockData in cache
	//

	foundBlock := programs.GetOrCreateDerivedBlockData(blockId1, flow.ZeroID)
	require.Same(t, block1, foundBlock)

	foundBlock = programs.Get(blockId1)
	require.Same(t, block1, foundBlock)

	entry = block1.programs.GetForTestingOnly(loc1)
	require.NotNil(t, entry)
	require.Same(t, prog1, entry.Value)

	// writes to block2 did't poplute block1.
	entry = block1.programs.GetForTestingOnly(loc2)
	require.Nil(t, entry)

	//
	// Test eviction
	//

	blockId3 := testBlockId(3)

	// block3 forces block1 to evict
	block3 := programs.GetOrCreateDerivedBlockData(blockId3, blockId2)
	require.NotNil(t, block3)
	require.NotSame(t, block1, block3)
	require.NotSame(t, block2, block3)

	entry = block3.programs.GetForTestingOnly(loc1)
	require.NotNil(t, entry)
	require.Same(t, prog1, entry.Value)

	entry = block3.programs.GetForTestingOnly(loc2)
	require.NotNil(t, entry)
	require.Same(t, prog2, entry.Value)

	// block1 forces block2 to evict
	foundBlock = programs.GetOrCreateDerivedBlockData(blockId1, flow.ZeroID)
	require.NotSame(t, block1, foundBlock)

	entry = foundBlock.programs.GetForTestingOnly(loc1)
	require.Nil(t, entry)

	entry = foundBlock.programs.GetForTestingOnly(loc2)
	require.Nil(t, entry)

	block1 = foundBlock

	//
	// Scripts don't poplute the cache
	//

	// Create from cached current block
	scriptBlock := programs.NewDerivedBlockDataForScript(blockId3)
	require.NotNil(t, scriptBlock)
	require.NotSame(t, block2, scriptBlock)
	require.NotSame(t, block3, scriptBlock)

	entry = scriptBlock.programs.GetForTestingOnly(loc1)
	require.NotNil(t, entry)
	require.Same(t, prog1, entry.Value)

	entry = scriptBlock.programs.GetForTestingOnly(loc2)
	require.NotNil(t, entry)
	require.Same(t, prog2, entry.Value)

	foundBlock = programs.Get(blockId3)
	require.Same(t, block3, foundBlock)

	foundBlock = programs.Get(blockId1)
	require.Same(t, block1, foundBlock)

	foundBlock = programs.Get(blockId2)
	require.Nil(t, foundBlock)
}
