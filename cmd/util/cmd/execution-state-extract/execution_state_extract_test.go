package extract

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExtractExecutionState(t *testing.T) {
	metr := &metrics.NoopCollector{}

	t.Run("missing block->state commitment mapping", func(t *testing.T) {

		withDirs(t, func(datadir, execdir, outdir string) {
			db := common.InitStorage(datadir)
			commits := badger.NewCommits(metr, db)

			_, err := getStateCommitment(commits, unittest.IdentifierFixture())
			require.Error(t, err)
		})
	})

	t.Run("retrieves block->state mapping", func(t *testing.T) {

		withDirs(t, func(datadir, execdir, outdir string) {
			db := common.InitStorage(datadir)
			commits := badger.NewCommits(metr, db)

			blockID := unittest.IdentifierFixture()
			stateCommitment := unittest.StateCommitmentFixture()

			err := commits.Store(blockID, stateCommitment)
			require.NoError(t, err)

			retrievedStateCommitment, err := getStateCommitment(commits, blockID)
			require.NoError(t, err)
			require.Equal(t, stateCommitment, retrievedStateCommitment)
		})
	})

	t.Run("empty WAL doesn't find anything", func(t *testing.T) {
		withDirs(t, func(datadir, execdir, outdir string) {
			err := extractExecutionState(execdir, unittest.StateCommitmentFixture(), outdir, zerolog.Nop())
			require.Error(t, err)
		})
	})

	t.Run("happy path", func(t *testing.T) {
		withDirs(t, func(datadir, execdir, _ string) {

			db := common.InitStorage(datadir)
			commits := badger.NewCommits(metr, db)

			// generate some ledger data
			size := 10
			keyByteSize := 32
			valueMaxByteSize := 1024

			f, err := ledger.NewMTrieStorage(execdir, size*10, metr, nil)
			require.NoError(t, err)

			var stateCommitment = f.EmptyStateCommitment()

			//saved data after updates
			keysValuesByCommit := make(map[string]map[string][]byte)
			commitsByBlocks := make(map[flow.Identifier][]byte)
			blocksInOrder := make([]flow.Identifier, size)

			for i := 0; i < size; i++ {
				keys := utils.GetRandomKeysFixedN(4, keyByteSize)
				values := utils.GetRandomValues(len(keys), 10, valueMaxByteSize)

				stateCommitment, err = f.UpdateRegisters(keys, values, stateCommitment)
				require.NoError(t, err)

				// generate random block and map it to state commitment
				blockID := unittest.IdentifierFixture()
				err := commits.Store(blockID, stateCommitment)
				require.NoError(t, err)

				data := make(map[string][]byte, len(keys))
				for j, key := range keys {
					data[string(key)] = values[j]
				}

				keysValuesByCommit[string(stateCommitment)] = data
				commitsByBlocks[blockID] = stateCommitment
				blocksInOrder[i] = blockID
			}

			<-f.Done()
			err = db.Close()
			require.NoError(t, err)

			//for blockID, stateCommitment := range commitsByBlocks {

			for i, blockID := range blocksInOrder {

				stateCommitment := commitsByBlocks[blockID]

				//we need fresh output dir to prevent contamination
				unittest.RunWithTempDir(t, func(outdir string) {

					Cmd.SetArgs([]string{"--execution-state-dir", execdir, "--output-dir", outdir, "--block-hash", blockID.String(), "--datadir", datadir})
					err := Cmd.Execute()
					require.NoError(t, err)

					require.FileExists(t, path.Join(outdir, "checkpoint.00000000")) //make sure we have checkpoint file

					//mForest, err := mtrie.NewMForest(ledger.MaxHeight, outdir, 1000, metr, func(evictedTrie *trie.MTrie) error {
					//	return nil
					//})
					//require.NoError(t, err)
					//
					//
					//w, err := wal.NewWAL(nil, nil, outdir, 1000, ledger.MaxHeight)
					//require.NoError(t, err)
					//
					//err = w.Replay(
					//	func(forestSequencing *flattener.FlattenedForest) error {
					//		rebuiltTries, err := flattener.RebuildTries(forestSequencing)
					//		if err != nil {
					//			return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
					//		}
					//		err = mForest.AddTries(rebuiltTries)
					//		if err != nil {
					//			return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
					//		}
					//		return nil
					//	},
					//	func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
					//		return fmt.Errorf("no WAL updates expected")
					//	},
					//	func(stateCommitment flow.StateCommitment) error {
					//		return fmt.Errorf("no WAL deletion expected")
					//	},
					//)

					storage, err := ledger.NewMTrieStorage(outdir, 1000, metr, nil)

					require.NoError(t, err)

					data := keysValuesByCommit[string(stateCommitment)]

					keys := make([][]byte, 0, len(data))
					for keyString := range data {
						key := []byte(keyString)
						keys = append(keys, key)
					}

					//registerValues, err := mForest.Read([]byte(stateCommitment), keys)
					registerValues, err := storage.GetRegisters(keys, []byte(stateCommitment))
					require.NoError(t, err)

					for i, key := range keys {
						registerValue := registerValues[i]

						require.Equal(t, data[string(key)], registerValue)
					}

					//make sure blocks after this one are not in checkpoint
					// ie - extraction stops after hitting right hash
					for j := i + 1; j < len(blocksInOrder); j++ {
						_, err := storage.GetRegisters(keys, commitsByBlocks[blocksInOrder[j]])
						require.Error(t, err)
					}

				})
			}
		})
	})
}

func withDirs(t *testing.T, f func(datadir, execdir, outdir string)) {
	unittest.RunWithTempDir(t, func(datadir string) {
		unittest.RunWithTempDir(t, func(exeDir string) {
			unittest.RunWithTempDir(t, func(outDir string) {
				f(datadir, exeDir, outDir)
			})
		})
	})
}
