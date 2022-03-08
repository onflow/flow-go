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
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// Compactor observer that waits until it gets notified of a
// latest checkpoint larger than fromBound
type CompactorObserver struct {
	fromBound int
	done      chan struct{}
}

func (co *CompactorObserver) OnNext(val interface{}) {
	res, ok := val.(int)
	if ok {
		new := res
		if new >= co.fromBound {
			co.done <- struct{}{}
		}
	}
}
func (co *CompactorObserver) OnError(err error) {}
func (co *CompactorObserver) OnComplete() {
	close(co.done)
}

func Test_Compactor(t *testing.T) {

	numInsPerStep := 2
	pathByteSize := 32
	minPayloadByteSize := 2 << 15
	maxPayloadByteSize := 2 << 16
	size := 10
	metricsCollector := &metrics.NoopCollector{}
	checkpointDistance := uint(2)

	unittest.RunWithTempDir(t, func(dir string) {

		f, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		var rootHash = f.GetEmptyRootHash()

		//saved data after updates
		savedData := make(map[ledger.RootHash]map[ledger.Path]*ledger.Payload)

		t.Run("Compactor creates checkpoints eventually", func(t *testing.T) {

			wal, err := NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments

			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			compactor := NewCompactor(checkpointer, 100*time.Millisecond, checkpointDistance, 1, zerolog.Nop()) //keep only latest checkpoint
			co := CompactorObserver{fromBound: 9, done: make(chan struct{})}
			compactor.Subscribe(&co)

			// Run Compactor in background.
			<-compactor.Ready()

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				paths0 := utils.RandomPaths(numInsPerStep)
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

				data := make(map[ledger.Path]*ledger.Payload, len(paths))
				for j, path := range paths {
					data[path] = payloads[j]
				}

				savedData[rootHash] = data
			}

			// wait for the bound-checking observer to confirm checkpoints have been made
			select {
			case <-co.done:
				// continue
			case <-time.After(60 * time.Second):
				assert.FailNow(t, "timed out")
			}

			from, to, err := checkpointer.NotCheckpointedSegments()
			require.NoError(t, err)

			assert.True(t, from == 10 && to == 10, "from: %v, to: %v", from, to) //make sure there is no leftover

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
			<-wal.Done()
			require.NoError(t, err)
		})

		time.Sleep(2 * time.Second)

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

		f2, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		t.Run("load data from checkpoint and WAL", func(t *testing.T) {
			wal2, err := NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			err = wal2.Replay(
				func(tries []*trie.MTrie) error {
					return f2.AddTries(tries)
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

			<-wal2.Done()

		})

		t.Run("make sure forests are equal", func(t *testing.T) {

			//check for same data
			for rootHash, data := range savedData {

				paths := make([]ledger.Path, 0, len(data))
				for path := range data {
					paths = append(paths, path)
				}

				read := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
				payloads, err := f.Read(read)
				require.NoError(t, err)

				payloads2, err := f2.Read(read)
				require.NoError(t, err)

				for i, path := range paths {
					require.True(t, data[path].Equals(payloads[i]))
					require.True(t, data[path].Equals(payloads2[i]))
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

		f, err := mtrie.NewForest(size*10, metricsCollector, nil)
		require.NoError(t, err)

		var rootHash = f.GetEmptyRootHash()

		t.Run("Compactor creates checkpoints", func(t *testing.T) {

			wal, err := NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, size*10, pathByteSize, 32*1024)
			require.NoError(t, err)

			// WAL segments are 32kB, so here we generate 2 keys 64kB each, times `size`
			// so we should get at least `size` segments
			checkpointer, err := wal.NewCheckpointer()
			require.NoError(t, err)

			compactor := NewCompactor(checkpointer, 100*time.Millisecond, checkpointDistance, 2, zerolog.Nop())

			// Generate the tree and create WAL
			for i := 0; i < size; i++ {

				paths0 := utils.RandomPaths(numInsPerStep)
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
				_, err = compactor.createCheckpoints()
				require.NoError(t, err)
			}

			// assert creation of checkpoint files precisely
			require.NoFileExists(t, path.Join(dir, bootstrap.FilenameWALRootCheckpoint))
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

			require.NoFileExists(t, path.Join(dir, bootstrap.FilenameWALRootCheckpoint))
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

			<-wal.Done()
			require.NoError(t, err)
		})
	})
}
