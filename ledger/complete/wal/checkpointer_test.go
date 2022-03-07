package wal_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	numInsPerStep      = 2
	keyNumberOfParts   = 10
	keyPartMinByteSize = 1
	keyPartMaxByteSize = 100
	valueMaxByteSize   = 2 << 16 //16kB
	size               = 10
	metricsCollector   = &metrics.NoopCollector{}
	logger             = zerolog.Logger{}
	segmentSize        = 32 * 1024
	pathByteSize       = 32
	pathFinderVersion  = uint8(complete.DefaultPathFinderVersion)
)

func Test_WAL(t *testing.T) {

	unittest.RunWithTempDir(t, func(dir string) {

		diskWal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, size, pathfinder.PathByteSize, realWAL.SegmentSize)
		require.NoError(t, err)

		led, err := complete.NewLedger(diskWal, size*10, metricsCollector, logger, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		var state = led.InitialState()

		//saved data after updates
		savedData := make(map[string]map[string]ledger.Value)

		// WAL segments are 32kB, so here we generate 2 keys 16kB each, times `size`
		// so we should get at least `size` segments

		for i := 0; i < size; i++ {

			keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
			values := utils.RandomValues(numInsPerStep, valueMaxByteSize/2, valueMaxByteSize)
			update, err := ledger.NewUpdate(state, keys, values)
			require.NoError(t, err)
			state, _, err = led.Set(update)
			require.NoError(t, err)

			fmt.Printf("Updated with %x\n", state)

			data := make(map[string]ledger.Value, len(keys))
			for j, key := range keys {
				data[string(encoding.EncodeKey(&key))] = values[j]
			}

			savedData[string(state[:])] = data
		}

		<-diskWal.Done()
		<-led.Done()

		diskWal2, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dir, size, pathfinder.PathByteSize, realWAL.SegmentSize)
		require.NoError(t, err)
		led2, err := complete.NewLedger(diskWal2, (size*10)+10, metricsCollector, logger, complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		// random map iteration order is a benefit here
		for state, data := range savedData {

			keys := make([]ledger.Key, 0, len(data))
			for keyString := range data {
				key, err := encoding.DecodeKey([]byte(keyString))
				require.NoError(t, err)
				keys = append(keys, *key)
			}

			fmt.Printf("Querying with %x\n", state)

			var ledgerState ledger.State
			copy(ledgerState[:], state)
			query, err := ledger.NewQuery(ledgerState, keys)
			require.NoError(t, err)
			values, err := led2.Get(query)
			require.NoError(t, err)

			for i, key := range keys {
				assert.Equal(t, data[string(encoding.EncodeKey(&key))], values[i])
			}
		}

		<-diskWal2.Done()
		<-led2.Done()
	})
}

func Test_Checkpointing(t *testing.T) {

	unittest.RunWithTempDir(t, func(dir string) {

		f, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		var rootHash = f.GetEmptyRootHash()

		//saved data after updates
		savedData := make(map[ledger.RootHash]map[ledger.Path]*ledger.Payload)

		t.Run("create WAL and initial trie", func(t *testing.T) {

			wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, 1600, 1600)
				values := utils.RandomValues(numInsPerStep, valueMaxByteSize/2, valueMaxByteSize)
				update, err := ledger.NewUpdate(ledger.State(rootHash), keys, values)
				require.NoError(t, err)

				trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, pathFinderVersion)
				require.NoError(t, err)

				err = wal.RecordUpdate(trieUpdate)
				require.NoError(t, err)

				rootHash, err := f.Update(trieUpdate)
				require.NoError(t, err)

				fmt.Printf("Updated with %x\n", rootHash)

				data := make(map[ledger.Path]*ledger.Payload, len(trieUpdate.Paths))
				for j, path := range trieUpdate.Paths {
					data[path] = trieUpdate.Payloads[j]
				}

				savedData[rootHash] = data
			}
			// some buffer time of the checkpointer to run
			time.Sleep(1 * time.Second)
			<-wal.Done()

			require.FileExists(t, path.Join(dir, "00000010")) //make sure we have enough segments saved
		})

		// create a new forest and replay WAL
		f2, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		t.Run("replay WAL and create checkpoint", func(t *testing.T) {

			require.NoFileExists(t, path.Join(dir, "checkpoint.00000010"))

			wal2, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
			require.NoError(t, err)

			err = wal2.Replay(
				func(tries []*trie.MTrie) error {
					return fmt.Errorf("I should fail as there should be no checkpoints")
				},
				func(update *ledger.TrieUpdate) error {
					_, err := f2.Update(update)
					return err
				},
				func(rootHash ledger.RootHash) error {
					return fmt.Errorf("I should fail as there should be no deletions")
				},
			)
			require.NoError(t, err)

			checkpointer, err := wal2.NewCheckpointer()
			require.NoError(t, err)

			err = checkpointer.Checkpoint(10, func() (io.WriteCloser, error) {
				return checkpointer.CheckpointWriter(10)
			})
			require.NoError(t, err)

			require.FileExists(t, path.Join(dir, "checkpoint.00000010")) //make sure we have checkpoint file

			<-wal2.Done()
		})

		f3, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		t.Run("read checkpoint", func(t *testing.T) {
			wal3, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
			require.NoError(t, err)

			err = wal3.Replay(
				func(tries []*trie.MTrie) error {
					return f3.AddTries(tries)
				},
				func(update *ledger.TrieUpdate) error {
					return fmt.Errorf("I should fail as there should be no updates")
				},
				func(rootHash ledger.RootHash) error {
					return fmt.Errorf("I should fail as there should be no deletions")
				},
			)
			require.NoError(t, err)

			<-wal3.Done()
		})

		t.Run("all forests contain the same data", func(t *testing.T) {
			// random map iteration order is a benefit here
			// make sure the tries has been rebuilt from WAL and another from from Checkpoint
			// f1, f2 and f3 should be identical
			for rootHash, data := range savedData {

				paths := make([]ledger.Path, 0, len(data))
				for path := range data {
					paths = append(paths, path)
				}

				payloads1, err := f.Read(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
				require.NoError(t, err)

				payloads2, err := f2.Read(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
				require.NoError(t, err)

				payloads3, err := f3.Read(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
				require.NoError(t, err)

				for i, path := range paths {
					require.True(t, data[path].Equals(payloads1[i]))
					require.True(t, data[path].Equals(payloads2[i]))
					require.True(t, data[path].Equals(payloads3[i]))
				}
			}
		})

		keys2 := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
		values2 := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)
		t.Run("create segment after checkpoint", func(t *testing.T) {

			//require.NoFileExists(t, path.Join(dir, "00000011"))

			unittest.RequireFileEmpty(t, path.Join(dir, "00000011"))

			//generate one more segment
			wal4, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
			require.NoError(t, err)

			update, err := ledger.NewUpdate(ledger.State(rootHash), keys2, values2)
			require.NoError(t, err)

			trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, pathFinderVersion)
			require.NoError(t, err)

			err = wal4.RecordUpdate(trieUpdate)
			require.NoError(t, err)

			rootHash, err = f.Update(trieUpdate)
			require.NoError(t, err)

			<-wal4.Done()

			require.FileExists(t, path.Join(dir, "00000011")) //make sure we have extra segment
		})

		f5, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		t.Run("replay both checkpoint and updates after checkpoint", func(t *testing.T) {
			wal5, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
			require.NoError(t, err)

			updatesLeft := 1 // there should be only one update

			err = wal5.Replay(
				func(tries []*trie.MTrie) error {
					return f5.AddTries(tries)
				},
				func(update *ledger.TrieUpdate) error {
					if updatesLeft == 0 {
						return fmt.Errorf("more updates called then expected")
					}
					_, err := f5.Update(update)
					updatesLeft--
					return err
				},
				func(rootHash ledger.RootHash) error {
					return fmt.Errorf("I should fail as there should be no deletions")
				},
			)
			require.NoError(t, err)

			<-wal5.Done()
		})

		t.Run("extra updates were applied correctly", func(t *testing.T) {

			query, err := ledger.NewQuery(ledger.State(rootHash), keys2)
			require.NoError(t, err)
			trieRead, err := pathfinder.QueryToTrieRead(query, pathFinderVersion)
			require.NoError(t, err)

			payloads, err := f.Read(trieRead)
			require.NoError(t, err)

			payloads5, err := f5.Read(trieRead)
			require.NoError(t, err)

			for i := range keys2 {
				require.Equal(t, values2[i], payloads[i].Value)
				require.Equal(t, values2[i], payloads5[i].Value)
			}
		})

		t.Run("corrupted checkpoints are skipped", func(t *testing.T) {

			f6, err := mtrie.NewForest(size*10, metricsCollector, nil)
			require.NoError(t, err)

			wal6, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
			require.NoError(t, err)

			// make sure no earlier checkpoints exist
			require.NoFileExists(t, path.Join(dir, "checkpoint.0000008"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.0000006"))
			require.NoFileExists(t, path.Join(dir, "checkpoint.0000004"))

			require.FileExists(t, path.Join(dir, "checkpoint.00000010"))

			// create missing checkpoints
			checkpointer, err := wal6.NewCheckpointer()
			require.NoError(t, err)

			err = checkpointer.Checkpoint(4, func() (io.WriteCloser, error) {
				return checkpointer.CheckpointWriter(4)
			})
			require.NoError(t, err)
			require.FileExists(t, path.Join(dir, "checkpoint.00000004"))

			err = checkpointer.Checkpoint(6, func() (io.WriteCloser, error) {
				return checkpointer.CheckpointWriter(6)
			})
			require.NoError(t, err)
			require.FileExists(t, path.Join(dir, "checkpoint.00000006"))

			err = checkpointer.Checkpoint(8, func() (io.WriteCloser, error) {
				return checkpointer.CheckpointWriter(8)
			})
			require.NoError(t, err)
			require.FileExists(t, path.Join(dir, "checkpoint.00000008"))

			// corrupt checkpoints
			randomlyModifyFile(t, path.Join(dir, "checkpoint.00000006"))
			randomlyModifyFile(t, path.Join(dir, "checkpoint.00000008"))
			randomlyModifyFile(t, path.Join(dir, "checkpoint.00000010"))

			// make sure 10 is latest checkpoint
			latestCheckpoint, err := checkpointer.LatestCheckpoint()
			require.NoError(t, err)
			require.Equal(t, 10, latestCheckpoint)

			// at this stage, number 4 should be the latest valid checkpoint
			// check other fail to load

			_, err = checkpointer.LoadCheckpoint(10)
			require.Error(t, err)
			_, err = checkpointer.LoadCheckpoint(8)
			require.Error(t, err)
			_, err = checkpointer.LoadCheckpoint(6)
			require.Error(t, err)
			_, err = checkpointer.LoadCheckpoint(4)
			require.NoError(t, err)

			err = wal6.ReplayOnForest(f6)
			require.NoError(t, err)

			<-wal6.Done()

			// check if the latest data is still there
			query, err := ledger.NewQuery(ledger.State(rootHash), keys2)
			require.NoError(t, err)
			trieRead, err := pathfinder.QueryToTrieRead(query, pathFinderVersion)
			require.NoError(t, err)

			payloads, err := f.Read(trieRead)
			require.NoError(t, err)

			payloads6, err := f6.Read(trieRead)
			require.NoError(t, err)

			for i := range keys2 {
				require.Equal(t, values2[i], payloads[i].Value)
				require.Equal(t, values2[i], payloads6[i].Value)
			}

		})
	})
}

func TestCheckpointFileError(t *testing.T) {

	unittest.RunWithTempDir(t, func(dir string) {

		wal, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
		require.NoError(t, err)

		// create WAL

		keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, 1600, 1600)
		values := utils.RandomValues(numInsPerStep, valueMaxByteSize/2, valueMaxByteSize)
		update, err := ledger.NewUpdate(ledger.State(trie.EmptyTrieRootHash()), keys, values)
		require.NoError(t, err)

		trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, pathFinderVersion)
		require.NoError(t, err)

		err = wal.RecordUpdate(trieUpdate)
		require.NoError(t, err)

		// some buffer time of the checkpointer to run
		time.Sleep(1 * time.Second)
		<-wal.Done()

		require.FileExists(t, path.Join(dir, "00000001")) //make sure WAL segment is saved

		wal2, err := realWAL.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, segmentSize)
		require.NoError(t, err)

		checkpointer, err := wal2.NewCheckpointer()
		require.NoError(t, err)

		t.Run("write error", func(t *testing.T) {
			errWrite := errors.New("unexpected write error")

			err = checkpointer.Checkpoint(1, func() (io.WriteCloser, error) {
				return newWriteCloserWithErrors(errWrite, nil), nil
			})
			require.ErrorIs(t, err, errWrite)
		})

		t.Run("close error", func(t *testing.T) {
			errClose := errors.New("unexpected close error")

			err = checkpointer.Checkpoint(1, func() (io.WriteCloser, error) {
				return newWriteCloserWithErrors(nil, errClose), nil
			})
			require.ErrorIs(t, err, errClose)
		})
	})
}

// randomlyModifyFile picks random byte and modifies it
// this should be enough to cause checkpoint loading to fail
// as it contains checksum
func randomlyModifyFile(t *testing.T, filename string) {

	file, err := os.OpenFile(filename, os.O_RDWR, 0644)
	require.NoError(t, err)

	fileInfo, err := file.Stat()
	require.NoError(t, err)

	fileSize := fileInfo.Size()

	buf := make([]byte, 1)

	// get some random offset
	offset := int64(rand.Int()) % (fileSize + int64(len(buf)))

	_, err = file.ReadAt(buf, offset)
	require.NoError(t, err)

	// byte addition will simply wrap around
	buf[0]++

	_, err = file.WriteAt(buf, offset)
	require.NoError(t, err)
}

func Test_StoringLoadingCheckpoints(t *testing.T) {

	unittest.RunWithTempDir(t, func(dir string) {
		// some hash will be literally encoded in output file
		// so we can find it and modify - to make sure we get a different checksum
		// but not fail process by, for example, modifying saved data length causing EOF

		emptyTrie := trie.NewEmptyMTrie()

		p1 := utils.PathByUint8(0)
		v1 := utils.LightPayload8('A', 'a')

		p2 := utils.PathByUint8(1)
		v2 := utils.LightPayload8('B', 'b')

		paths := []ledger.Path{p1, p2}
		payloads := []ledger.Payload{*v1, *v2}

		updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)

		someHash := updatedTrie.RootNode().LeftChild().Hash() // Hash of left child

		file, err := ioutil.TempFile(dir, "temp-checkpoint")
		filepath := file.Name()
		require.NoError(t, err)

		err = realWAL.StoreCheckpoint(file, updatedTrie)
		require.NoError(t, err)

		file.Close()

		t.Run("works without data modification", func(t *testing.T) {
			tries, err := realWAL.LoadCheckpoint(filepath)
			require.NoError(t, err)
			require.Equal(t, 1, len(tries))
			require.Equal(t, updatedTrie, tries[0])
		})

		t.Run("detects modified data", func(t *testing.T) {
			b, err := ioutil.ReadFile(filepath)
			require.NoError(t, err)

			index := bytes.Index(b, someHash[:])
			require.NotEqual(t, -1, index)
			b[index] = 23

			err = os.WriteFile(filepath, b, 0644)
			require.NoError(t, err)

			tries, err := realWAL.LoadCheckpoint(filepath)
			require.Error(t, err)
			require.Nil(t, tries)
			require.Contains(t, err.Error(), "checksum")
		})
	})
}

type writeCloserWithErrors struct {
	writeError error
	closeError error
}

func newWriteCloserWithErrors(writeError error, closeError error) *writeCloserWithErrors {
	return &writeCloserWithErrors{
		writeError: writeError,
		closeError: closeError,
	}
}

func (wc *writeCloserWithErrors) Write(p []byte) (n int, err error) {
	return 0, wc.writeError
}

func (wc *writeCloserWithErrors) Close() error {
	return wc.closeError
}
