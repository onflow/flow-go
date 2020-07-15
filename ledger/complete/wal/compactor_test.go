package wal_test

// import (
// 	"fmt"
// 	"os"
// 	"path"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"

// 	"github.com/dapperlabs/flow-go/ledger"
// 	"github.com/dapperlabs/flow-go/ledger/common"
// 	"github.com/dapperlabs/flow-go/ledger/complete/mtrie"
// 	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/flattener"
// 	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/trie"
// 	"github.com/dapperlabs/flow-go/module/metrics"
// 	"github.com/dapperlabs/flow-go/utils/unittest"
// )

// // TODO Ramtin merge master into this
// func Test_Compactor(t *testing.T) {

// 	numInsPerStep := 2
// 	pathByteSize := 4
// 	// TODO use this
// 	// payloadMaxByteSize := 2 << 16 //64kB
// 	size := 10
// 	metricsCollector := &metrics.NoopCollector{}

// 	unittest.RunWithTempDir(t, func(dir string) {

// 		f, err := mtrie.NewForest(4, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
// 		require.NoError(t, err)

// 		var rootHash = f.GetEmptyRootHash()

// 		//saved data after updates
// 		savedData := make(map[string]map[string]*ledger.Payload)

// 		t.Run("Compactor creates checkpoints eventually", func(t *testing.T) {

// 			wal, err := NewWAL(nil, nil, dir, size*10, 4)
// 			require.NoError(t, err)

// 			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
// 			// so we should get at least `size` segments

// 			checkpointer, err := wal.NewCheckpointer()
// 			require.NoError(t, err)

// 			compactor := NewCompactor(checkpointer, 100*time.Millisecond)

// 			// Run Compactor in background.
// 			<-compactor.Ready()

// 			// Generate the tree and create WAL
// 			for i := 0; i < size; i++ {

// 				paths0 := common.GetRandomPaths(numInsPerStep, pathByteSize)
// 				payloads0 := common.RandomPayloads(numInsPerStep)
// 				paths := make([]ledger.Path, 0)
// 				payloads := make([]*ledger.Payload, 0)
// 				paths = append(paths, paths0...)
// 				payloads = append(payloads, payloads0...)

// 				update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
// 				err = wal.RecordUpdate(update)
// 				require.NoError(t, err)

// 				rootHash, err := f.Update(update)
// 				require.NoError(t, err)

// 				require.FileExists(t, path.Join(dir, numberToFilenamePart(i)))

// 				data := make(map[string]*ledger.Payload, len(paths))
// 				for j, path := range paths {
// 					data[string(path)] = payloads[j]
// 				}

// 				savedData[string(rootHash)] = data
// 			}

// 			assert.Eventually(t, func() bool {
// 				from, to, err := checkpointer.NotCheckpointedSegments()
// 				require.NoError(t, err)

// 				return to == from && from == 10 //make sure there is only one segment ahead of checkpoint
// 				// this is disk-based operation after all, so give it big timeout
// 			}, 15*time.Second, 100*time.Millisecond)

// 			require.FileExists(t, path.Join(dir, "checkpoint.00000009"))

// 			<-compactor.Done()
// 			err = wal.Close()
// 			require.NoError(t, err)
// 		})

// 		t.Run("remove unnecessary files", func(t *testing.T) {
// 			// Remove all files apart from target checkpoint and WAL segments ahead of it
// 			// We know their names, so just hardcode them
// 			dirF, _ := os.Open(dir)
// 			files, _ := dirF.Readdir(0)

// 			for _, fileInfo := range files {

// 				name := fileInfo.Name()

// 				if name != "checkpoint.00000009" && name != "00000010" {
// 					err := os.Remove(path.Join(dir, name))
// 					require.NoError(t, err)
// 				}
// 			}
// 		})

// 		f2, err := mtrie.NewForest(4, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
// 		require.NoError(t, err)

// 		t.Run("load data from checkpoint and WAL", func(t *testing.T) {
// 			wal2, err := NewWAL(nil, nil, dir, size*10, 4)
// 			require.NoError(t, err)

// 			err = wal2.Replay(
// 				func(forestSequencing *flattener.FlattenedForest) error {
// 					return loadIntoForest(f2, forestSequencing)
// 				},
// 				func(update *ledger.TrieUpdate) error {
// 					_, err := f2.Update(update)
// 					return err
// 				},
// 				func(rootHash ledger.RootHash) error {
// 					return fmt.Errorf("no deletion expected")
// 				},
// 			)
// 			require.NoError(t, err)

// 			err = wal2.Close()
// 			require.NoError(t, err)
// 		})

// 		t.Run("make sure forests are equal", func(t *testing.T) {

// 			//check for same data
// 			for rootHash, data := range savedData {

// 				paths := make([]ledger.Path, 0, len(data))
// 				for pathString := range data {
// 					path := []byte(pathString)
// 					paths = append(paths, path)
// 				}

// 				read := &ledger.TrieRead{RootHash: []byte(rootHash), Paths: paths}
// 				registerValues, err := f.Read(read)
// 				require.NoError(t, err)

// 				registerValues2, err := f2.Read(read)
// 				require.NoError(t, err)

// 				for i, path := range paths {
// 					require.Equal(t, data[string(path)], registerValues[i])
// 					require.Equal(t, data[string(path)], registerValues2[i])
// 				}
// 			}

// 			// check for
// 			forestTries, err := f.GetTries()
// 			require.NoError(t, err)

// 			forestTries2, err := f2.GetTries()
// 			require.NoError(t, err)

// 			// order might be different
// 			require.Equal(t, len(forestTries), len(forestTries2))
// 		})

// 	})
// }

// func loadIntoForest(forest *mtrie.Forest, forestSequencing *flattener.FlattenedForest) error {
// 	tries, err := flattener.RebuildTries(forestSequencing)
// 	if err != nil {
// 		return err
// 	}
// 	for _, t := range tries {
// 		err := forest.AddTrie(t)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
