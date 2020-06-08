package wal

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/sequencer"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func Test_Compactor(t *testing.T) {

	numInsPerStep := 2
	keyByteSize := 4
	valueMaxByteSize := 2 << 16 //64kB
	size := 10
	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		f, err := mtrie.NewMForest(33, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
		require.NoError(t, err)

		var stateCommitment = f.GetEmptyRootHash()

		//saved data after updates
		savedData := make(map[string]map[string][]byte)

		t.Run("Compactor creates checkpoints eventually", func(t *testing.T) {

			wal, err := NewWAL(nil, nil, dir, size*10, 33)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			checkpointer, err := wal.Checkpointer()
			require.NoError(t, err)

			compactor := NewCompactor(checkpointer, 5*time.Millisecond)

			// Run Compactor in background.
			<-compactor.Ready()

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
				values := utils.GetRandomValues(len(keys), valueMaxByteSize, valueMaxByteSize)

				err = wal.RecordUpdate(stateCommitment, keys, values)
				require.NoError(t, err)

				newTrie, err := f.Update(stateCommitment, keys, values)
				stateCommitment := newTrie.RootHash()
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, numberToFilenamePart(i)))

				data := make(map[string][]byte, len(keys))
				for j, key := range keys {
					data[string(key)] = values[j]
				}

				savedData[string(stateCommitment)] = data
			}

			assert.Eventually(t, func() bool {
				from, to, err := checkpointer.NotCheckpointedSegments()
				require.NoError(t, err)

				return to == from && from == 10 //make sure there is only one segment ahead of checkpoint
			}, 2000*time.Millisecond, 100*time.Millisecond)

			require.FileExists(t, path.Join(dir, "checkpoint.00000009"))

			<-compactor.Done()
			err = wal.Close()
			require.NoError(t, err)
		})

		t.Run("remove unnecessary files", func(t *testing.T) {
			// Remove all files apart from target checkpoint and WAL segments ahead of it
			// We know their names, so just hardcode them
			dirF, _ := os.Open(dir)
			files, _ := dirF.Readdir(0)

			for _, fileInfo := range files {

				name := fileInfo.Name()

				if name != "checkpoint.00000009" && name != "00000010" {
					err := os.Remove(path.Join(dir, name))
					require.NoError(t, err)
				}
			}
		})

		f2, err := mtrie.NewMForest(33, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
		require.NoError(t, err)

		t.Run("load data from checkpoint and WAL", func(t *testing.T) {
			wal2, err := NewWAL(nil, nil, dir, size*10, 33)
			require.NoError(t, err)

			err = wal2.Replay(
				func(nodes []*sequencer.StorableNode, tries []*sequencer.StorableTrie) error {
					return loadIntoForest(f2, nodes, tries)
				},
				func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
					_, err := f2.Update(commitment, keys, values)
					return err
				},
				func(commitment flow.StateCommitment) error {
					return fmt.Errorf("no deletion expected")
				},
			)
			require.NoError(t, err)

			err = wal2.Close()
			require.NoError(t, err)
		})

		t.Run("make sure forests are equal", func(t *testing.T) {

			//check for same data
			for stateCommitment, data := range savedData {

				keys := make([][]byte, 0, len(data))
				for keyString := range data {
					key := []byte(keyString)
					keys = append(keys, key)
				}

				registerValues, err := f.Read([]byte(stateCommitment), keys)
				require.NoError(t, err)

				registerValues2, err := f2.Read([]byte(stateCommitment), keys)
				require.NoError(t, err)

				for i, key := range keys {
					require.Equal(t, data[string(key)], registerValues[i])
					require.Equal(t, data[string(key)], registerValues2[i])
				}
			}

			// check for
			forestTries, err := f.GetTries()
			require.NoError(t, err)

			forestTries2, err := f2.GetTries()
			require.NoError(t, err)

			// order might be different
			require.Equal(t, len(forestTries), len(forestTries2))
		})

	})
}

func loadIntoForest(forest *mtrie.MForest, storableNodes []*sequencer.StorableNode, storableTries []*sequencer.StorableTrie) error {
	forestSequencing := &sequencer.MForestSequencing{Nodes: storableNodes, Tries: storableTries}
	tries, err := sequencer.RebuildTries(forestSequencing)
	if err != nil {
		return err
	}
	for _, t := range tries {
		err := forest.AddTrie(t)
		if err != nil {
			return err
		}
	}
	return nil
}
