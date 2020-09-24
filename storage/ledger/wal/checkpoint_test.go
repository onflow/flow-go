package wal_test

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/ledger/mtrie/flattener"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/ledger"
	"github.com/onflow/flow-go/storage/ledger/mtrie"
	"github.com/onflow/flow-go/storage/ledger/mtrie/trie"
	"github.com/onflow/flow-go/storage/ledger/utils"
	realWAL "github.com/onflow/flow-go/storage/ledger/wal"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func RunWithWALCheckpointerWithFiles(t *testing.T, names ...interface{}) {
	f := names[len(names)-1].(func(*testing.T, *realWAL.LedgerWAL, *realWAL.Checkpointer))

	fileNames := make([]string, len(names)-1)

	for i := 0; i <= len(names)-2; i++ {
		fileNames[i] = names[i].(string)
	}

	unittest.RunWithTempDir(t, func(dir string) {
		util.CreateFiles(t, dir, fileNames...)

		wal, err := realWAL.NewWAL(nil, nil, dir, 10, 1, segmentSize)
		require.NoError(t, err)

		checkpointer, err := wal.NewCheckpointer()
		require.NoError(t, err)

		f(t, wal, checkpointer)
	})
}

var (
	numInsPerStep    = 2
	keyByteSize      = 4
	valueMaxByteSize = 2 << 16 //64kB
	size             = 10
	metricsCollector = &metrics.NoopCollector{}
	segmentSize      = 32 * 1024
)

func Test_WAL(t *testing.T) {

	keyByteSize := 32

	unittest.RunWithTempDir(t, func(dir string) {

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

			registerValues, err := f2.GetRegisters(keys, []byte(stateCommitment))
			require.NoError(t, err)

			for i, key := range keys {
				registerValue := registerValues[i]

				assert.Equal(t, data[string(key)], registerValue)
			}
		}

		<-f2.Done()

	})
}

func Test_Checkpointing(t *testing.T) {

	unittest.RunWithTempDir(t, func(dir string) {
		unittest.RunWithTempDir(t, func(dirRoot string) {

			f, err := mtrie.NewMForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
			require.NoError(t, err)

			var stateCommitment = f.GetEmptyRootHash()

			//saved data after updates
			savedData := make(map[string]map[string][]byte)

			t.Run("create WAL and initial trie", func(t *testing.T) {

				wal, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
				// so we should get at least `size` segments

				// Generate the tree and create WAL
				for i := 0; i < size; i++ {

					keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
					values := utils.GetRandomValues(len(keys), valueMaxByteSize, valueMaxByteSize)

					err = wal.RecordUpdate(stateCommitment, keys, values)
					require.NoError(t, err)

					newTrie, err := f.Update(stateCommitment, keys, values)
					stateCommitment := newTrie.RootHash()
					require.NoError(t, err)

					data := make(map[string][]byte, len(keys))
					for j, key := range keys {
						data[string(key)] = values[j]
					}

					savedData[string(stateCommitment)] = data
				}
				err = wal.Close()
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, "00000010")) //make sure we have enough segments saved
			})

			// create a new forest and reply WAL
			f2, err := mtrie.NewMForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
			require.NoError(t, err)

			t.Run("replay WAL and create checkpoints", func(t *testing.T) {

				require.NoFileExists(t, path.Join(dir, "checkpoint.00000010"))
				require.NoFileExists(t, path.Join(dir, "checkpoint.00000007"))
				require.NoFileExists(t, path.Join(dir, "checkpoint.00000005"))

				wal2, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				err = wal2.Replay(
					func(forestSequencing *flattener.FlattenedForest) error {
						return fmt.Errorf("I should fail as there should be no checkpoints")
					},
					func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
						_, err := f2.Update(commitment, keys, values)
						return err
					},
					func(commitment flow.StateCommitment) error {
						return fmt.Errorf("I should fail as there should be no deletions")
					},
				)
				require.NoError(t, err)

				checkpointer, err := wal2.NewCheckpointer()
				require.NoError(t, err)

				// create checkpoint at 5 to use it as a root
				err = checkpointer.Checkpoint(5, func() (io.WriteCloser, error) {
					return checkpointer.CheckpointWriter(5)
				})
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, "checkpoint.00000005"))

				// create checkpoint at 8 to use it for root loading tests
				err = checkpointer.Checkpoint(8, func() (io.WriteCloser, error) {
					return checkpointer.CheckpointWriter(8)
				})
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, "checkpoint.00000008"))

				err = checkpointer.Checkpoint(10, func() (io.WriteCloser, error) {
					return checkpointer.CheckpointWriter(10)
				})
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, "checkpoint.00000010")) //make sure we have checkpoint file

				err = copyFile(path.Join(dir, "checkpoint.00000005"), path.Join(dirRoot, realWAL.RootCheckpointFilename))
				require.NoError(t, err)
				// copy rest of WALs
				err = copyFile(path.Join(dir, "00000006"), path.Join(dirRoot, "00000000"))
				require.NoError(t, err)
				err = copyFile(path.Join(dir, "00000007"), path.Join(dirRoot, "00000001"))
				require.NoError(t, err)
				err = copyFile(path.Join(dir, "00000008"), path.Join(dirRoot, "00000002"))
				require.NoError(t, err)
				err = copyFile(path.Join(dir, "00000009"), path.Join(dirRoot, "00000003"))
				require.NoError(t, err)
				err = copyFile(path.Join(dir, "00000010"), path.Join(dirRoot, "00000004"))
				require.NoError(t, err)

				err = wal2.Close()
				require.NoError(t, err)
			})

			f3, err := mtrie.NewMForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
			require.NoError(t, err)

			t.Run("read checkpoint", func(t *testing.T) {
				wal3, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				err = wal3.Replay(
					func(forestSequencing *flattener.FlattenedForest) error {
						return loadIntoForest(f3, forestSequencing)
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
			})

			fRoot1, err := mtrie.NewMForest(keyByteSize, dirRoot, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
			require.NoError(t, err)

			t.Run("read root checkpoint", func(t *testing.T) {
				walRoot1, err := realWAL.NewWAL(nil, nil, dirRoot, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				err = walRoot1.Replay(
					func(forestSequencing *flattener.FlattenedForest) error {
						return loadIntoForest(fRoot1, forestSequencing)
					},
					func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
						_, err := fRoot1.Update(commitment, keys, values)
						return err
					},
					func(commitment flow.StateCommitment) error {
						return fmt.Errorf("I should fail as there should be no deletions")
					},
				)
				require.NoError(t, err)

				err = walRoot1.Close()
				require.NoError(t, err)
			})

			fRoot2, err := mtrie.NewMForest(keyByteSize, dirRoot, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
			require.NoError(t, err)

			t.Run("root checkpoint isn't read when other checkpoints is loaded", func(t *testing.T) {

				//copy some later checkpoint
				err = copyFile(path.Join(dir, "checkpoint.00000008"), path.Join(dirRoot, "checkpoint.00000002"))
				require.NoError(t, err)

				//scramble root checkpoint to make sure its not read
				err = scrambleFile(path.Join(dirRoot, realWAL.RootCheckpointFilename))
				require.NoError(t, err)

				walRoot2, err := realWAL.NewWAL(nil, nil, dirRoot, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				err = walRoot2.Replay(
					func(forestSequencing *flattener.FlattenedForest) error {
						return loadIntoForest(fRoot2, forestSequencing)
					},
					func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
						_, err := fRoot2.Update(commitment, keys, values)
						return err
					},
					func(commitment flow.StateCommitment) error {
						return fmt.Errorf("I should fail as there should be no deletions")
					},
				)
				require.NoError(t, err)

				err = walRoot2.Close()
				require.NoError(t, err)
			})

			t.Run("all forests contain the same data", func(t *testing.T) {
				// random map iteration order is a benefit here
				// make sure the tries has been rebuilt from WAL and another from from Checkpoint
				// f1, f2 and f3 should be identical
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

					registerValues3, err := f3.Read([]byte(stateCommitment), keys)
					require.NoError(t, err)

					registerValuesR1, err := fRoot1.Read([]byte(stateCommitment), keys)
					require.NoError(t, err)

					registerValuesR2, err := fRoot2.Read([]byte(stateCommitment), keys)
					require.NoError(t, err)

					for i, key := range keys {
						require.Equal(t, data[string(key)], registerValues[i])
						require.Equal(t, data[string(key)], registerValues2[i])
						require.Equal(t, data[string(key)], registerValues3[i])
						require.Equal(t, data[string(key)], registerValuesR1[i])
						require.Equal(t, data[string(key)], registerValuesR2[i])
					}
				}
			})

			keys2 := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
			values2 := utils.GetRandomValues(len(keys2), valueMaxByteSize, valueMaxByteSize)

			t.Run("create segment after checkpoint", func(t *testing.T) {

				require.NoFileExists(t, path.Join(dir, "00000011"))

				//generate one more segment
				wal4, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				err = wal4.RecordUpdate(stateCommitment, keys2, values2)
				require.NoError(t, err)

				newTrie, err := f.Update(stateCommitment, keys2, values2)
				require.NoError(t, err)
				stateCommitment = newTrie.RootHash()

				err = wal4.Close()
				require.NoError(t, err)

				require.FileExists(t, path.Join(dir, "00000011")) //make sure we have extra segment
			})

			f5, err := mtrie.NewMForest(keyByteSize, dir, size*10, metricsCollector, func(tree *trie.MTrie) error { return nil })
			require.NoError(t, err)

			t.Run("replay both checkpoint and updates after checkpoint", func(t *testing.T) {
				wal5, err := realWAL.NewWAL(nil, nil, dir, size*10, keyByteSize, segmentSize)
				require.NoError(t, err)

				updatesLeft := 1 // there should be only one update

				err = wal5.Replay(
					func(forestSequencing *flattener.FlattenedForest) error {
						return loadIntoForest(f5, forestSequencing)
					},
					func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
						if updatesLeft == 0 {
							return fmt.Errorf("more updates called then expected")
						}
						_, err := f5.Update(commitment, keys, values)
						updatesLeft--
						return err
					},
					func(commitment flow.StateCommitment) error {
						return fmt.Errorf("I should fail as there should be no deletions")
					},
				)
				require.NoError(t, err)

				err = wal5.Close()
				require.NoError(t, err)
			})

			t.Run("extra updates were applied correctly", func(t *testing.T) {
				registerValues, err := f.Read([]byte(stateCommitment), keys2)
				require.NoError(t, err)

				registerValues5, err := f5.Read([]byte(stateCommitment), keys2)
				require.NoError(t, err)

				for i := range keys2 {
					require.Equal(t, values2[i], registerValues[i])
					require.Equal(t, values2[i], registerValues5[i])
				}
			})
		})

	})
}

// TestCheckpointsVersions is a test for compatibility with previous binary versions of checkpoints
// Currently, we support both v1 and v2, but this is likely to evolve.
// Current version of checkpoints is tested with other tests.
// `checkpoints` directory contains sample checkpoints made by the code deployed in past environment
// but they aim to generate the same structure, for size-sake.
// Generated checkpoint start from empty and should contain 10 tries with 4-bytes keys and 4 byte values.
// Every tree adds two new entries - key is BigEndian encoded sequential uint32 (starting from 0),
// while values are 4 repeated bytes of increased value starting with ASCII '0'
// First update =>  0001 -> '0000', 0002 -> '1111'
// Second update => 0003 -> '2222', 0004 -> '3333'
// and so on.
func TestCheckpointsVersions(t *testing.T) {
	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {

		modelForest, err := mtrie.NewMForest(4, dir, ledger.CacheSize, metricsCollector, nil)
		require.NoError(t, err)

		counter := uint32(0)
		val := byte('0')

		stateCommitment := modelForest.GetEmptyRootHash()

		stateCommitments := make([][]byte, 10)
		allKeys := make([][][]byte, 10)

		for i := 0; i < 10; i++ {
			keys := make([][]byte, 2)
			values := make([][]byte, 2)
			for j := 0; j < 2; j++ {

				key := make([]byte, 4)
				binary.BigEndian.PutUint32(key, counter)
				keys[j] = key

				value := []byte{val, val, val, val}
				values[j] = value

				counter++
				val++
			}
			allKeys[i] = keys

			mTrie, err := modelForest.Update(stateCommitment, keys, values)
			require.NoError(t, err)

			stateCommitment = mTrie.RootHash()
			stateCommitments[i] = stateCommitment
		}

		t.Run("V1", func(t *testing.T) {

			mForest, err := mtrie.NewMForest(4, dir, ledger.CacheSize, metricsCollector, nil)
			require.NoError(t, err)

			flattenedForest, err := realWAL.LoadCheckpoint("checkpoints/checkpoint.00000000.v1")
			require.NoError(t, err)

			err = loadIntoForest(mForest, flattenedForest)
			require.NoError(t, err)

			for i, stateCommitment := range stateCommitments {
				modelTrie, err := modelForest.GetTrie(stateCommitment)
				require.NoError(t, err)

				mTrie, err := mForest.GetTrie(stateCommitment)
				require.NoError(t, err)

				keys := allKeys[i]

				modelValues, err := modelTrie.UnsafeRead(keys)
				require.NoError(t, err)

				mValues, err := mTrie.UnsafeRead(keys)
				require.NoError(t, err)

				assert.Equal(t, modelValues, mValues)
			}
		})
	})
}

func loadIntoForest(forest *mtrie.MForest, forestSequencing *flattener.FlattenedForest) error {
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

func copyFile(src, dst string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}
	return nil
}

func scrambleFile(f string) error {
	to, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = to.WriteString("scramblescramble")
	return err
}
