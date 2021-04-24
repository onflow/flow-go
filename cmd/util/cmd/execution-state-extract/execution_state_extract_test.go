package extract

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"

	"github.com/onflow/flow-go/utils/unittest"
)

type keyPair struct {
	key   ledger.Key
	value ledger.Value
}

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

			// generate some oldLedger data
			size := 10
			keyMaxByteSize := 64
			valueMaxByteSize := 1024

			diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), execdir, size, pathfinder.PathByteSize, wal.SegmentSize)
			require.NoError(t, err)
			f, err := complete.NewLedger(diskWal, size*10, metr, zerolog.Nop(), complete.DefaultPathFinderVersion)
			require.NoError(t, err)

			var stateCommitment = f.InitialState()

			//saved data after updates
			keysValuesByCommit := make(map[string]map[string]keyPair)
			commitsByBlocks := make(map[flow.Identifier]ledger.State)
			blocksInOrder := make([]flow.Identifier, size)

			for i := 0; i < size; i++ {
				//keys := utils.GetRandomRegisterIDs(4)
				//values := utils.GetRandomValues(len(keys), 10, valueMaxByteSize)

				keys := utils.RandomUniqueKeys(4, 3, 10, keyMaxByteSize)
				values := utils.RandomValues(len(keys), 10, valueMaxByteSize)

				update, err := ledger.NewUpdate(stateCommitment, keys, values)
				require.NoError(t, err)

				stateCommitment, err = f.Set(update)
				//stateCommitment, err = f.UpdateRegisters(keys, values, stateCommitment)
				require.NoError(t, err)

				// generate random block and map it to state commitment
				blockID := unittest.IdentifierFixture()
				err = commits.Store(blockID, flow.StateCommitment(stateCommitment))
				require.NoError(t, err)

				data := make(map[string]keyPair, len(keys))
				for j, key := range keys {
					data[key.String()] = keyPair{
						key:   key,
						value: values[j],
					}
				}

				keysValuesByCommit[string(stateCommitment[:])] = data
				commitsByBlocks[blockID] = stateCommitment
				blocksInOrder[i] = blockID
			}

			<-diskWal.Done()
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

					require.FileExists(t, path.Join(outdir, wal.RootCheckpointFilename)) //make sure we have root checkpoint file

					diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), outdir, size, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)

					storage, err := complete.NewLedger(diskWal, 1000, metr, zerolog.Nop(), complete.DefaultPathFinderVersion)
					require.NoError(t, err)

					data := keysValuesByCommit[string(stateCommitment[:])]

					keys := make([]ledger.Key, 0, len(data))
					for _, v := range data {
						keys = append(keys, v.key)
					}

					query, err := ledger.NewQuery(stateCommitment, keys)
					require.NoError(t, err)

					registerValues, err := storage.Get(query)
					//registerValues, err := mForest.Read([]byte(stateCommitment), keys)
					require.NoError(t, err)

					for i, key := range keys {
						registerValue := registerValues[i]

						require.Equal(t, data[key.String()].value, registerValue)
					}

					//make sure blocks after this one are not in checkpoint
					// ie - extraction stops after hitting right hash
					for j := i + 1; j < len(blocksInOrder); j++ {

						query.SetState(commitsByBlocks[blocksInOrder[j]])
						_, err := storage.Get(query)
						//_, err := storage.GetRegisters(keys, commitsByBlocks[blocksInOrder[j]])
						require.Error(t, err)
					}

					<-diskWal.Done()
					<-storage.Done()
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
