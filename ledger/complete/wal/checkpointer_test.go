package wal_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete"
	realWAL "github.com/dapperlabs/flow-go/ledger/complete/wal"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// "fmt"
// "io"
// "path"
// "testing"

// "github.com/stretchr/testify/assert"
// "github.com/stretchr/testify/require"

// "github.com/dapperlabs/flow-go/ledger/complete/mtrie/flattener"

// "github.com/dapperlabs/flow-go/ledger/complete"
// "github.com/dapperlabs/flow-go/ledger/complete/mtrie"
// "github.com/dapperlabs/flow-go/ledger/complete/mtrie/trie"
// realWAL "github.com/dapperlabs/flow-go/ledger/complete/wal"
// "github.com/dapperlabs/flow-go/ledger/utils"
// "github.com/dapperlabs/flow-go/model/flow"
// "github.com/dapperlabs/flow-go/module/metrics"
// "github.com/dapperlabs/flow-go/storage/util"
// "github.com/dapperlabs/flow-go/utils/unittest"

func RunWithWALCheckpointerWithFiles(t *testing.T, names ...interface{}) {
	f := names[len(names)-1].(func(*testing.T, *realWAL.LedgerWAL, *realWAL.Checkpointer))

	fileNames := make([]string, len(names)-1)

	for i := 0; i <= len(names)-2; i++ {
		fileNames[i] = names[i].(string)
	}

	unittest.RunWithTempDir(t, func(dir string) {
		util.CreateFiles(t, dir, fileNames...)

		wal, err := realWAL.NewWAL(nil, nil, dir, 10, 1)
		require.NoError(t, err)

		checkpointer, err := wal.NewCheckpointer()
		require.NoError(t, err)

		f(t, wal, checkpointer)
	})
}

func Test_WAL(t *testing.T) {

	numInsPerStep := 2
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 100 // 2 << 16 //16kB
	size := 10              // 10
	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		fmt.Println(dir)
		led, err := complete.NewLedger(dir, size*10, metricsCollector, nil)
		require.NoError(t, err)

		var stateCommitment = led.EmptyStateCommitment()

		//saved data after updates
		savedData := make(map[string]map[string]ledger.Value)

		// WAL segments are 32kB, so here we generate 2 keys 16kB each, times `size`
		// so we should get at least `size` segments
		for i := 0; i < size; i++ {

			keys := common.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
			values := common.RandomValues(numInsPerStep, 1, valueMaxByteSize)
			update, err := ledger.NewUpdate(stateCommitment, keys, values)
			require.NoError(t, err)
			stateCommitment, err = led.Set(update)
			require.NoError(t, err)

			fmt.Printf("Updated with %x\n", stateCommitment)

			data := make(map[string]ledger.Value, len(keys))
			for j, key := range keys {
				data[string(common.EncodeKey(&key))] = values[j]
			}

			savedData[string(stateCommitment)] = data
		}

		<-led.Done()

		led2, err := complete.NewLedger(dir, (size*10)+10, metricsCollector, nil)
		require.NoError(t, err)

		// random map iteration order is a benefit here
		for stateCommitment, data := range savedData {

			keys := make([]ledger.Key, 0, len(data))
			for keyString := range data {
				key, err := common.DecodeKey([]byte(keyString))
				require.NoError(t, err)
				keys = append(keys, *key)
			}

			fmt.Printf("Querying with %x\n", stateCommitment)

			query, err := ledger.NewQuery([]byte(stateCommitment), keys)

			values, err := led2.Get(query)
			require.NoError(t, err)

			for i, key := range keys {
				assert.Equal(t, data[string(common.EncodeKey(&key))], values[i])
			}
		}

		<-led2.Done()

	})
}

// func Test_Checkpointing(t *testing.T) {

// 	numInsPerStep := 2
// 	keyByteSize := 4
// 	valueMaxByteSize := 2 << 16 //64kB
// 	size := 10
// 	metricsCollector := &metrics.NoopCollector{}

// 	unittest.RunWithTempDir(t, func(dir string) {

// 		f, err := mtrie.NewForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
// 		require.NoError(t, err)

// 		var stateCommitment = f.GetEmptyRootHash()

// 		//saved data after updates
// 		savedData := make(map[string]map[string][]byte)

// 		t.Run("create WAL and initial trie", func(t *testing.T) {

// 			wal, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize)
// 			require.NoError(t, err)

// 			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
// 			// so we should get at least `size` segments

// 			// Generate the tree and create WAL
// 			for i := 0; i < size; i++ {

// 				keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
// 				values := utils.GetRandomValues(len(keys), valueMaxByteSize, valueMaxByteSize)

// 				err = wal.RecordUpdate(stateCommitment, keys, values)
// 				require.NoError(t, err)

// 				newTrie, err := f.Update(stateCommitment, keys, values)
// 				stateCommitment := newTrie.RootHash()
// 				require.NoError(t, err)

// 				fmt.Printf("Updated with %x\n", stateCommitment)

// 				data := make(map[string][]byte, len(keys))
// 				for j, key := range keys {
// 					data[string(key)] = values[j]
// 				}

// 				savedData[string(stateCommitment)] = data
// 			}
// 			err = wal.Close()
// 			require.NoError(t, err)

// 			require.FileExists(t, path.Join(dir, "00000010")) //make sure we have enough segments saved
// 		})

// 		// create a new forest and reply WAL
// 		f2, err := mtrie.NewForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
// 		require.NoError(t, err)

// 		t.Run("replay WAL and create checkpoint", func(t *testing.T) {

// 			require.NoFileExists(t, path.Join(dir, "checkpoint.00000010"))

// 			wal2, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize)
// 			require.NoError(t, err)

// 			err = wal2.Replay(
// 				func(forestSequencing *flattener.FlattenedForest) error {
// 					return fmt.Errorf("I should fail as there should be no checkpoints")
// 				},
// 				func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
// 					_, err := f2.Update(commitment, keys, values)
// 					return err
// 				},
// 				func(commitment flow.StateCommitment) error {
// 					return fmt.Errorf("I should fail as there should be no deletions")
// 				},
// 			)
// 			require.NoError(t, err)

// 			checkpointer, err := wal2.NewCheckpointer()
// 			require.NoError(t, err)

// 			err = checkpointer.Checkpoint(10, func() (io.WriteCloser, error) {
// 				return checkpointer.CheckpointWriter(10)
// 			})
// 			require.NoError(t, err)

// 			require.FileExists(t, path.Join(dir, "checkpoint.00000010")) //make sure we have checkpoint file

// 			err = wal2.Close()
// 			require.NoError(t, err)
// 		})

// 		f3, err := mtrie.NewForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
// 		require.NoError(t, err)

// 		t.Run("read checkpoint", func(t *testing.T) {
// 			wal3, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize)
// 			require.NoError(t, err)

// 			err = wal3.Replay(
// 				func(forestSequencing *flattener.FlattenedForest) error {
// 					return loadIntoForest(f3, forestSequencing)
// 				},
// 				func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
// 					return fmt.Errorf("I should fail as there should be no updates")
// 				},
// 				func(commitment flow.StateCommitment) error {
// 					return fmt.Errorf("I should fail as there should be no deletions")
// 				},
// 			)
// 			require.NoError(t, err)

// 			err = wal3.Close()
// 			require.NoError(t, err)
// 		})

// 		t.Run("all forests contain the same data", func(t *testing.T) {
// 			// random map iteration order is a benefit here
// 			// make sure the tries has been rebuilt from WAL and another from from Checkpoint
// 			// f1, f2 and f3 should be identical
// 			for stateCommitment, data := range savedData {

// 				keys := make([][]byte, 0, len(data))
// 				for keyString := range data {
// 					key := []byte(keyString)
// 					keys = append(keys, key)
// 				}

// 				fmt.Printf("Querying with %x\n", stateCommitment)

// 				registerValues, err := f.Read([]byte(stateCommitment), keys)
// 				require.NoError(t, err)

// 				registerValues2, err := f2.Read([]byte(stateCommitment), keys)
// 				require.NoError(t, err)

// 				registerValues3, err := f3.Read([]byte(stateCommitment), keys)
// 				require.NoError(t, err)

// 				for i, key := range keys {
// 					require.Equal(t, data[string(key)], registerValues[i])
// 					require.Equal(t, data[string(key)], registerValues2[i])
// 					require.Equal(t, data[string(key)], registerValues3[i])
// 				}
// 			}
// 		})

// 		keys2 := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
// 		values2 := utils.GetRandomValues(len(keys2), valueMaxByteSize, valueMaxByteSize)

// 		t.Run("create segment after checkpoint", func(t *testing.T) {

// 			require.NoFileExists(t, path.Join(dir, "00000011"))

// 			//generate one more segment
// 			wal4, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize)
// 			require.NoError(t, err)

// 			err = wal4.RecordUpdate(stateCommitment, keys2, values2)
// 			require.NoError(t, err)

// 			newTrie, err := f.Update(stateCommitment, keys2, values2)
// 			require.NoError(t, err)
// 			stateCommitment = newTrie.RootHash()

// 			err = wal4.Close()
// 			require.NoError(t, err)

// 			require.FileExists(t, path.Join(dir, "00000011")) //make sure we have extra segment
// 		})

// 		f5, err := mtrie.NewForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
// 		require.NoError(t, err)

// 		t.Run("replay both checkpoint and updates after checkpoint", func(t *testing.T) {
// 			wal5, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize)
// 			require.NoError(t, err)

// 			updatesLeft := 1 // there should be only one update

// 			err = wal5.Replay(
// 				func(forestSequencing *flattener.FlattenedForest) error {
// 					return loadIntoForest(f5, forestSequencing)
// 				},
// 				func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
// 					if updatesLeft == 0 {
// 						return fmt.Errorf("more updates called then expected")
// 					}
// 					_, err := f5.Update(commitment, keys, values)
// 					updatesLeft--
// 					return err
// 				},
// 				func(commitment flow.StateCommitment) error {
// 					return fmt.Errorf("I should fail as there should be no deletions")
// 				},
// 			)
// 			require.NoError(t, err)

// 			err = wal5.Close()
// 			require.NoError(t, err)
// 		})

// 		t.Run("extra updates were applied correctly", func(t *testing.T) {
// 			registerValues, err := f.Read([]byte(stateCommitment), keys2)
// 			require.NoError(t, err)

// 			registerValues5, err := f5.Read([]byte(stateCommitment), keys2)
// 			require.NoError(t, err)

// 			for i := range keys2 {
// 				require.Equal(t, values2[i], registerValues[i])
// 				require.Equal(t, values2[i], registerValues5[i])
// 			}
// 		})

// 	})
// }

// func loadIntoForest(forest *mtrie.MForest, forestSequencing *flattener.FlattenedForest) error {
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
