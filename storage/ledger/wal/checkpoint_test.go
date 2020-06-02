package wal_test

import (
	"fmt"
	"io"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	realWAL "github.com/dapperlabs/flow-go/storage/ledger/wal"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var dir = "./wal"

func checkpointerWithFiles(t *testing.T, names ...interface{}) {
	f := names[len(names)-1].(func(*testing.T, *realWAL.Checkpointer))

	fileNames := make([]string, len(names)-1)

	for i := 0; i <= len(names)-2; i++ {
		fileNames[i] = names[i].(string)
	}

	unittest.RunWithTempDir(t, func(dir string) {
		util.CreateFiles(t, dir, fileNames...)

		wal, err := realWAL.NewWAL(nil, nil, dir, 10, 9)
		require.NoError(t, err)

		checkpointer, err := wal.Checkpointer()
		require.NoError(t, err)

		f(t, checkpointer)
	})
}

func Test_WAL(t *testing.T) {

	//t.Skip()

	numInsPerStep := 2
	keyByteSize := 32
	valueMaxByteSize := 2 << 16 //16kB
	size := 10
	metricsCollector := &metrics.NoopCollector{}

	//unittest.RunWithTempDir(t, func(dir string) {

	f, err := ledger.NewMTrieStorage(dir, size*10, metricsCollector, nil)
	require.NoError(t, err)

	var stateCommitment = f.EmptyStateCommitment()

	//saved data after updates
	savedData := make(map[string]map[string][]byte)

	// WAL segments are 32kB, so here we generate 2 keys 16kB each, times `size`
	// so we should get at least `size` segments
	for i := 0; i < size; i++ {

		keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
		values := utils.GetRandomValues(len(keys), 10, valueMaxByteSize)

		stateCommitment, err = f.UpdateRegisters(keys, values, stateCommitment)
		require.NoError(t, err)

		fmt.Printf("Updated with %x\n", stateCommitment)

		data := make(map[string][]byte, len(keys))
		for j, key := range keys {
			data[string(key)] = values[j]
		}

		savedData[string(stateCommitment)] = data
	}

	<-f.Done()

	f2, err := ledger.NewMTrieStorage(dir, (size*10)+10, metricsCollector, nil)
	require.NoError(t, err)

	// random map iteration order is a benefit here
	for stateCommitment, data := range savedData {

		keys := make([][]byte, 0, len(data))
		for keyString := range data {
			key := []byte(keyString)
			keys = append(keys, key)
		}

		fmt.Printf("Querying with %x\n", stateCommitment)

		registerValues, err := f2.GetRegisters(keys, []byte(stateCommitment))
		require.NoError(t, err)

		for i, key := range keys {
			registerValue := registerValues[i]

			assert.Equal(t, data[string(key)], registerValue)
		}
	}

	<-f2.Done()

	//})
}

func Test_Checkpointing(t *testing.T) {

	//t.Skip()

	numInsPerStep := 2
	keyByteSize := 4
	valueMaxByteSize := 2 << 16 //16kB
	size := 10
	metricsCollector := &metrics.NoopCollector{}

	//unittest.RunWithTempDir(t, func(dir string) {

	//f, err := ledger.NewMTrieStorage(dir, size*10, metricsCollector, nil)
	f, err := mtrie.NewMForest(33, dir, size*10, metricsCollector, func(tree *mtrie.MTrie) error { return nil })
	require.NoError(t, err)

	wal, err := realWAL.NewWAL(nil, nil, dir, size*10, 33)
	require.NoError(t, err)

	var stateCommitment = f.GetEmptyRootHash()

	//saved data after updates
	savedData := make(map[string]map[string][]byte)

	// WAL segments are 32kB, so here we generate 2 keys 16kB each, times `size`
	// so we should get at least `size` segments

	// Generate the tree and create WAL
	for i := 0; i < size; i++ {

		keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
		values := utils.GetRandomValues(len(keys), valueMaxByteSize, valueMaxByteSize)

		err = wal.RecordUpdate(stateCommitment, keys, values)
		require.NoError(t, err)

		stateCommitment, err = f.Update(keys, values, stateCommitment)
		require.NoError(t, err)

		fmt.Printf("Updated with %x\n", stateCommitment)

		data := make(map[string][]byte, len(keys))
		for j, key := range keys {
			data[string(key)] = values[j]
		}

		savedData[string(stateCommitment)] = data
	}
	err = wal.Close()
	require.NoError(t, err)

	require.FileExists(t, path.Join(dir, "00000010")) //make sure we have enough segments saved

	// create a new forest and reply WAL
	f2, err := mtrie.NewMForest(33, dir, size*10, metricsCollector, func(tree *mtrie.MTrie) error { return nil })
	require.NoError(t, err)

	wal2, err := realWAL.NewWAL(nil, nil, dir, size*10, 33)
	require.NoError(t, err)

	err = wal2.Replay(
		func(nodes []*mtrie.StorableNode, tries []*mtrie.StorableTrie) error {
			return fmt.Errorf("I should fail as there should be no checkpoints")
		},
		func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			_, err := f2.Update(keys, values, commitment)
			return err
		},
		func(commitment flow.StateCommitment) error {
			return fmt.Errorf("I should fail as there should be no deletions")
		},
	)
	require.NoError(t, err)

	checkpointer, err := wal2.Checkpointer()
	require.NoError(t, err)

	err = checkpointer.Checkpoint(10, func() (io.WriteCloser, error) {
		return checkpointer.CheckpointWriter(10)
	})

	require.FileExists(t, path.Join(dir, "checkpoint.00000010")) //make sure we have checkpoint file

	err = wal2.Close()
	require.NoError(t, err)

	f3, err := mtrie.NewMForest(33, dir, size*10, metricsCollector, func(tree *mtrie.MTrie) error { return nil })
	require.NoError(t, err)

	wal3, err := realWAL.NewWAL(nil, nil, dir, size*10, 33)
	require.NoError(t, err)

	err = wal3.Replay(
		func(nodes []*mtrie.StorableNode, tries []*mtrie.StorableTrie) error {
			return f3.LoadStorables(nodes, tries)
		},
		func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			return fmt.Errorf("I should fail as there should be no updates")
		},
		func(commitment flow.StateCommitment) error {
			return fmt.Errorf("I should fail as there should be no deletions")
		},
	)
	require.NoError(t, err)

	err = wal3.Close()
	require.NoError(t, err)

	// random map iteration order is a benefit here
	// make sure the tries has been rebuilt from WAL and another from from Checkpoint
	// f1, f2 and f3 should be identical
	for stateCommitment, data := range savedData {

		keys := make([][]byte, 0, len(data))
		for keyString := range data {
			key := []byte(keyString)
			keys = append(keys, key)
		}

		fmt.Printf("Querying with %x\n", stateCommitment)

		registerValues, err := f.Read(keys, []byte(stateCommitment))
		require.NoError(t, err)

		registerValues2, err := f2.Read(keys, []byte(stateCommitment))
		require.NoError(t, err)

		registerValues3, err := f3.Read(keys, []byte(stateCommitment))
		require.NoError(t, err)

		for i, key := range keys {
			require.Equal(t, data[string(key)], registerValues[i])
			require.Equal(t, data[string(key)], registerValues2[i])
			require.Equal(t, data[string(key)], registerValues3[i])
		}
	}

	//})
}

func Test_emptyDir(t *testing.T) {
	checkpointerWithFiles(t, func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, -1, latestCheckpoint)

		// here when starting LedgerWAL, it creates empty file for writing, hence directory isn't really empty
		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 0, from)
		require.Equal(t, 0, to)
	})
}

// Prometheus WAL require files to be 8 characters, otherwise it gets confused

func Test_noCheckpoints(t *testing.T) {
	checkpointerWithFiles(t, "00000000", "00000001", "00000002", func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, -1, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 0, from)
		require.Equal(t, 2, to)
	})
}

func Test_someCheckpoints(t *testing.T) {
	checkpointerWithFiles(t, "00000000", "00000001", "00000002", "00000003", "00000004", "00000005", "checkpoint.00000002", func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 2, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 3, from)
		require.Equal(t, 5, to)
	})
}

func Test_loneCheckpoint(t *testing.T) {
	checkpointerWithFiles(t, "checkpoint.00000005", func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, -1, from)
		require.Equal(t, -1, to)
	})
}

func Test_checkpointWithoutPrecedingSegments(t *testing.T) {
	checkpointerWithFiles(t, "checkpoint.00000005", "00000006", "00000007", func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 6, from)
		require.Equal(t, 7, to)
	})
}

func Test_checkpointWithSameSegment(t *testing.T) {
	checkpointerWithFiles(t, "checkpoint.00000005", "00000005", "00000006", "00000007", func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 6, from)
		require.Equal(t, 7, to)
	})
}

func Test_NoGapBetweenSegmentsAndLastCheckpoint(t *testing.T) {
	checkpointerWithFiles(t, "checkpoint.00000004", "00000006", "00000007", func(t *testing.T, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 4, latestCheckpoint)

		_, _, err = checkpointer.NotCheckpointedSegments()
		require.Error(t, err)
	})
}

func Test_CheckpointCreation(t *testing.T) {
	wal, err := realWAL.NewWAL(nil, nil, dir, 10, 257)
	require.NoError(t, err)

	checkpointer, err := wal.Checkpointer()
	require.NoError(t, err)

	err = checkpointer.Checkpoint(5, func() (io.WriteCloser, error) {
		return checkpointer.CheckpointWriter(5)
	})
	require.NoError(t, err)
}
