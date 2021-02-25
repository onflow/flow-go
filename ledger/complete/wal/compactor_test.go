package wal

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_Compactor(t *testing.T) {

	numInsPerStep := 2
	pathByteSize := 32
	minPayloadByteSize := 2 << 15
	maxPayloadByteSize := 2 << 16
	size := 10
	metricsCollector := &metrics.NoopCollector{}
	checkpointDistance := uint(2)

	unittest.RunWithTempDir(t, func(dir string) {

		f, err := mtrie.NewForest(pathByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
		require.NoError(t, err)

		var rootHash = f.GetEmptyRootHash()

		//saved data after updates
		savedData := make(map[string]map[string]*ledger.Payload)

		t.Run("Compactor creates checkpoints eventually", func(t *testing.T) {

			wal, err := NewWAL(zerolog.Nop(), nil, dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			compactor := NewCompactor(checkpointer, 100*time.Millisecond, checkpointDistance, 1) //keep only latest checkpoint

			// Run Compactor in background.
			<-compactor.Ready()

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				paths0 := utils.RandomPaths(numInsPerStep, pathByteSize)
				payloads0 := utils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

				var paths []ledger.Path
				var payloads []*ledger.Payload
				paths = append(paths, paths0...)
				payloads = append(payloads, payloads0...)

				update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}

				err = wal.RecordUpdate(update)
				require.NoError(t, err)

				rootHash, err = f.Update(update)
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, NumberToFilenamePart(i)))

				data := make(map[string]*ledger.Payload, len(paths))
				for j, path := range paths {
					data[string(path)] = payloads[j]
				}

				savedData[string(rootHash)] = data
			}

			assert.Eventually(t, func() bool {
				from, to, err := checkpointer.NotCheckpointedSegments()
				require.NoError(t, err)

				return from == 10 && to == 10 //make sure there is
				// this is disk-based operation after all, so give it big timeout
			}, 15*time.Second, 100*time.Millisecond)

			require.NoFileExists(t, path.Join(dir, "checkpoint.00000000"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000001"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000002"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000003"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000004"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000005"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000006"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000007"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000008"))
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

				if name != "checkpoint.00000009" &&
					name != "00000010" {
					err := os.Remove(path.Join(dir, name))
					require.NoError(t, err)
				}
			}
		})

		f2, err := mtrie.NewForest(pathByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
		require.NoError(t, err)

		t.Run("load data from checkpoint and WAL", func(t *testing.T) {
			wal2, err := NewWAL(zerolog.Nop(), nil, dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			err = wal2.Replay(
				func(forestSequencing *flattener.FlattenedForest) error {
					return loadIntoForest(f2, forestSequencing)
				},
				func(update *ledger.TrieUpdate) error {
					_, err := f2.Update(update)
					return err
				},
				func(rootHash ledger.RootHash) error {
					return fmt.Errorf("no deletion expected")
				},
			)
			require.NoError(t, err)

			err = wal2.Close()
			require.NoError(t, err)
		})

		t.Run("make sure forests are equal", func(t *testing.T) {

			//check for same data
			for rootHash, data := range savedData {

				paths := make([]ledger.Path, 0, len(data))
				for pathString := range data {
					path := []byte(pathString)
					paths = append(paths, path)
				}

				read := &ledger.TrieRead{RootHash: ledger.RootHash(rootHash), Paths: paths}
				payloads, err := f.Read(read)
				require.NoError(t, err)

				payloads2, err := f2.Read(read)
				require.NoError(t, err)

				for i, path := range paths {
					require.True(t, data[string(path)].Equals(payloads[i]))
					require.True(t, data[string(path)].Equals(payloads2[i]))
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

func Test_Compactor_checkpointInterval(t *testing.T) {

	numInsPerStep := 2
	pathByteSize := 32
	minPayloadByteSize := 100
	maxPayloadByteSize := 2 << 16
	size := 20
	metricsCollector := &metrics.NoopCollector{}
	checkpointDistance := uint(3) // there should be 3 WAL not checkpointed

	unittest.RunWithTempDir(t, func(dir string) {

		f, err := mtrie.NewForest(pathByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
		require.NoError(t, err)

		var rootHash = f.GetEmptyRootHash()

		t.Run("Compactor creates checkpoints", func(t *testing.T) {

			wal, err := NewWAL(zerolog.Nop(), nil, dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments
			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			compactor := NewCompactor(checkpointer, 100*time.Millisecond, checkpointDistance, 2)

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				paths0 := utils.RandomPaths(numInsPerStep, pathByteSize)
				payloads0 := utils.RandomPayloads(numInsPerStep, minPayloadByteSize, maxPayloadByteSize)

				var paths []ledger.Path
				var payloads []*ledger.Payload
				// TODO figure out twice insert
				paths = append(paths, paths0...)
				payloads = append(payloads, payloads0...)

				update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}

				err = wal.RecordUpdate(update)
				require.NoError(t, err)

				rootHash, err = f.Update(update)
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, NumberToFilenamePart(i)))

				// run checkpoint creation after every file
				err = compactor.createCheckpoints()
				require.NoError(t, err)
			}

			// assert precisely creation of checkpoint files
			require.NoFileExists(t, path.Join(dir, RootCheckpointFilename))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000001"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000002"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000003"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000004"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000005"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000006"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000007"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000008"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000009"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000010"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000011"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000012"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000013"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000014"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000015"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000016"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000017"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000017"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000019"))

			// expect all but last 2 checkpoints gone
			err = compactor.cleanupCheckpoints()
			require.NoError(t, err)

			require.NoFileExists(t, path.Join(dir, RootCheckpointFilename))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000001"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000002"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000003"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000004"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000005"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000006"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000007"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000008"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000009"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000010"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000011"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000012"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000013"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000014"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000015"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000016"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000017"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.00000017"))
			require.FileExists(t, path.Join(dir, "checkpoint.00000019"))

			err = wal.Close()
			require.NoError(t, err)
		})
	})
}

func loadIntoForest(forest *mtrie.Forest, forestSequencing *flattener.FlattenedForest) error {
	tries, err := flattener.RebuildTries(forestSequencing)
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
