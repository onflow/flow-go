package state_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func TestStateDiff(t *testing.T) {
	offchainState, err := state.ImportEVMStateFromGob("/var/flow2/evm-state-from-gobs-218215348/")
	require.NoError(t, err)

	enState, err := state.ImportEVMStateFromGob("/var/flow2/evm-state-from-gobs-218215348/")
	require.NoError(t, err)

	differences := state.Diff(enState, offchainState)

	require.Len(t, differences, 0)
}

func TestEVMStateDiff(t *testing.T) {

	state1 := EVMStateFromReplayGobDir(t, "/var/flow2/evm-state-from-gobs-218215348/", uint64(218215348))
	// state2 := EVMStateFromReplayGobDir(t, "/var/flow2/evm-state-from-gobs-218215348/", uint64(218215348))
	state2 := EVMStateFromCheckpointExtract(t, "/var/flow2/evm-state-from-checkpoint-218215348/")

	differences := state.Diff(state1, state2)

	for i, diff := range differences {
		fmt.Printf("Difference %d: %v\n", i, diff)
	}

	require.Len(t, differences, 0)
}

func EVMStateFromCheckpointExtract(t *testing.T, dir string) *state.EVMState {
	enState, err := state.ImportEVMStateFromGob("/var/flow2/evm-state-from-gobs-218215348/")
	require.NoError(t, err)
	return enState
}

func EVMStateFromReplayGobDir(t *testing.T, gobDir string, flowHeight uint64) *state.EVMState {
	valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(gobDir, flowHeight)
	chainID := flow.Testnet

	allocatorGobs, err := testutils.DeserializeAllocator(allocatorFileName)
	require.NoError(t, err)

	storageRoot := evm.StorageAccountAddress(chainID)
	valuesGob, err := testutils.DeserializeState(valueFileName)
	require.NoError(t, err)

	store := testutils.GetSimpleValueStorePopulated(valuesGob, allocatorGobs)

	bv, err := state.NewBaseView(store, storageRoot)
	require.NoError(t, err)

	evmState, err := state.Extract(storageRoot, bv)
	require.NoError(t, err)
	return evmState
}

func evmStateGobFileNamesByEndHeight(evmStateGobDir string, endHeight uint64) (string, string) {
	valueFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("values-%d.gob", endHeight))
	allocatorFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("allocators-%d.gob", endHeight))
	return valueFileName, allocatorFileName
}
