package extract

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	runtimeCommon "github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
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

			_, err := commits.ByBlockID(unittest.IdentifierFixture())
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

			retrievedStateCommitment, err := commits.ByBlockID(blockID)
			require.NoError(t, err)
			require.Equal(t, stateCommitment, retrievedStateCommitment)
		})
	})

	t.Run("empty WAL doesn't find anything", func(t *testing.T) {
		withDirs(t, func(datadir, execdir, outdir string) {
			opts := migrations.Options{
				NWorker:              10,
				ChainID:              flow.Emulator,
				EVMContractChange:    migrations.EVMContractChangeNone,
				BurnerContractChange: migrations.BurnerContractChangeDeploy,
				VerboseErrorOutput:   true,
			}

			err := extractExecutionState(
				zerolog.Nop(),
				execdir,
				unittest.StateCommitmentFixture(),
				outdir,
				10,
				false,
				"",
				nil,
				false,
				opts,
			)
			require.Error(t, err)
		})
	})

	t.Run("happy path", func(t *testing.T) {

		withDirs(t, func(datadir, execdir, _ string) {

			const (
				checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
				checkpointsToKeep  = 1
			)

			db := common.InitStorage(datadir)
			commits := badger.NewCommits(metr, db)

			// generate some oldLedger data
			size := 10

			diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), execdir, size, pathfinder.PathByteSize, wal.SegmentSize)
			require.NoError(t, err)
			f, err := complete.NewLedger(diskWal, size*10, metr, zerolog.Nop(), complete.DefaultPathFinderVersion)
			require.NoError(t, err)
			compactor, err := complete.NewCompactor(f, diskWal, zerolog.Nop(), uint(size), checkpointDistance, checkpointsToKeep, atomic.NewBool(false), &metrics.NoopCollector{})
			require.NoError(t, err)
			<-compactor.Ready()

			var stateCommitment = f.InitialState()

			// saved data after updates
			keysValuesByCommit := make(map[string]map[string]keyPair)
			commitsByBlocks := make(map[flow.Identifier]ledger.State)
			blocksInOrder := make([]flow.Identifier, size)

			for i := 0; i < size; i++ {
				keys, values := getSampleKeyValues(i)

				update, err := ledger.NewUpdate(stateCommitment, keys, values)
				require.NoError(t, err)

				stateCommitment, _, err = f.Set(update)
				// stateCommitment, err = f.UpdateRegisters(keys, values, stateCommitment)
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

			<-f.Done()
			<-compactor.Done()

			err = db.Close()
			require.NoError(t, err)

			// for blockID, stateCommitment := range commitsByBlocks {

			for i, blockID := range blocksInOrder {

				stateCommitment := commitsByBlocks[blockID]

				// we need fresh output dir to prevent contamination
				unittest.RunWithTempDir(t, func(outdir string) {

					Cmd.SetArgs([]string{
						"--execution-state-dir", execdir,
						"--output-dir", outdir,
						"--state-commitment", stateCommitment.String(),
						"--datadir", datadir,
						"--no-migration",
						"--no-report",
						"--chain", flow.Emulator.Chain().String()})

					err := Cmd.Execute()
					require.NoError(t, err)

					diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), outdir, size, pathfinder.PathByteSize, wal.SegmentSize)
					require.NoError(t, err)

					storage, err := complete.NewLedger(diskWal, 1000, metr, zerolog.Nop(), complete.DefaultPathFinderVersion)
					require.NoError(t, err)

					const (
						checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
						checkpointsToKeep  = 1
					)
					compactor, err := complete.NewCompactor(storage, diskWal, zerolog.Nop(), uint(size), checkpointDistance, checkpointsToKeep, atomic.NewBool(false), &metrics.NoopCollector{})
					require.NoError(t, err)

					<-compactor.Ready()

					data := keysValuesByCommit[string(stateCommitment[:])]

					keys := make([]ledger.Key, 0, len(data))
					for _, v := range data {
						keys = append(keys, v.key)
					}

					query, err := ledger.NewQuery(stateCommitment, keys)
					require.NoError(t, err)

					registerValues, err := storage.Get(query)
					// registerValues, err := mForest.Read([]byte(stateCommitment), keys)
					require.NoError(t, err)

					for i, key := range keys {
						registerValue := registerValues[i]
						require.Equal(t, data[key.String()].value, registerValue)
					}

					// make sure blocks after this one are not in checkpoint
					// ie - extraction stops after hitting right hash
					for j := i + 1; j < len(blocksInOrder); j++ {

						query.SetState(commitsByBlocks[blocksInOrder[j]])
						_, err := storage.Get(query)
						require.Error(t, err)
					}

					<-storage.Done()
					<-compactor.Done()
				})
			}
		})
	})
}

// TestExtractPayloadsFromExecutionState tests state extraction with checkpoint as input and payload as output.
func TestExtractPayloadsFromExecutionState(t *testing.T) {
	metr := &metrics.NoopCollector{}

	const payloadFileName = "root.payload"

	t.Run("all payloads", func(t *testing.T) {
		withDirs(t, func(_, execdir, outdir string) {

			const (
				checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
				checkpointsToKeep  = 1
			)

			outputPayloadFileName := filepath.Join(outdir, payloadFileName)

			size := 10

			diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), execdir, size, pathfinder.PathByteSize, wal.SegmentSize)
			require.NoError(t, err)
			f, err := complete.NewLedger(diskWal, size*10, metr, zerolog.Nop(), complete.DefaultPathFinderVersion)
			require.NoError(t, err)
			compactor, err := complete.NewCompactor(f, diskWal, zerolog.Nop(), uint(size), checkpointDistance, checkpointsToKeep, atomic.NewBool(false), &metrics.NoopCollector{})
			require.NoError(t, err)
			<-compactor.Ready()

			var stateCommitment = f.InitialState()

			// Save generated data after updates
			keysValues := make(map[string]keyPair)

			for i := 0; i < size; i++ {
				keys, values := getSampleKeyValues(i)

				update, err := ledger.NewUpdate(stateCommitment, keys, values)
				require.NoError(t, err)

				stateCommitment, _, err = f.Set(update)
				require.NoError(t, err)

				for j, key := range keys {
					keysValues[key.String()] = keyPair{
						key:   key,
						value: values[j],
					}
				}
			}

			<-f.Done()
			<-compactor.Done()

			tries, err := f.Tries()
			require.NoError(t, err)

			err = wal.StoreCheckpointV6SingleThread(tries, execdir, "checkpoint.00000001", zerolog.Nop())
			require.NoError(t, err)

			// Export all payloads
			Cmd.SetArgs([]string{
				"--execution-state-dir", execdir,
				"--output-dir", outdir,
				"--state-commitment", hex.EncodeToString(stateCommitment[:]),
				"--no-migration",
				"--no-report",
				"--output-payload-filename", outputPayloadFileName,
				"--chain", flow.Emulator.Chain().String()})

			err = Cmd.Execute()
			require.NoError(t, err)

			// Verify exported payloads.
			partialState, payloadsFromFile, err := util.ReadPayloadFile(zerolog.Nop(), outputPayloadFileName)
			require.NoError(t, err)
			require.Equal(t, len(keysValues), len(payloadsFromFile))
			require.False(t, partialState)

			for _, payloadFromFile := range payloadsFromFile {
				k, err := payloadFromFile.Key()
				require.NoError(t, err)

				kv, exist := keysValues[k.String()]
				require.True(t, exist)
				require.Equal(t, kv.value, payloadFromFile.Value())
			}
		})
	})

	t.Run("some payloads", func(t *testing.T) {
		withDirs(t, func(_, execdir, outdir string) {
			const (
				checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
				checkpointsToKeep  = 1
			)

			outputPayloadFileName := filepath.Join(outdir, payloadFileName)

			size := 10

			diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), execdir, size, pathfinder.PathByteSize, wal.SegmentSize)
			require.NoError(t, err)
			f, err := complete.NewLedger(diskWal, size*10, metr, zerolog.Nop(), complete.DefaultPathFinderVersion)
			require.NoError(t, err)
			compactor, err := complete.NewCompactor(f, diskWal, zerolog.Nop(), uint(size), checkpointDistance, checkpointsToKeep, atomic.NewBool(false), &metrics.NoopCollector{})
			require.NoError(t, err)
			<-compactor.Ready()

			var stateCommitment = f.InitialState()

			// Save generated data after updates
			keysValues := make(map[string]keyPair)

			for i := 0; i < size; i++ {
				keys, values := getSampleKeyValues(i)

				update, err := ledger.NewUpdate(stateCommitment, keys, values)
				require.NoError(t, err)

				stateCommitment, _, err = f.Set(update)
				require.NoError(t, err)

				for j, key := range keys {
					keysValues[key.String()] = keyPair{
						key:   key,
						value: values[j],
					}
				}
			}

			<-f.Done()
			<-compactor.Done()

			tries, err := f.Tries()
			require.NoError(t, err)

			err = wal.StoreCheckpointV6SingleThread(tries, execdir, "checkpoint.00000001", zerolog.Nop())
			require.NoError(t, err)

			const selectedAddressCount = 10
			selectedAddresses := make(map[string]struct{})
			selectedKeysValues := make(map[string]keyPair)
			for k, kv := range keysValues {
				owner := kv.key.KeyParts[0].Value
				if len(owner) != runtimeCommon.AddressLength {
					continue
				}

				address, err := runtimeCommon.BytesToAddress(owner)
				require.NoError(t, err)

				if len(selectedAddresses) < selectedAddressCount {
					selectedAddresses[address.Hex()] = struct{}{}
				}

				if _, exist := selectedAddresses[address.Hex()]; exist {
					selectedKeysValues[k] = kv
				}
			}

			addresses := make([]string, 0, len(selectedAddresses))
			for address := range selectedAddresses {
				addresses = append(addresses, address)
			}

			// Export selected payloads
			Cmd.SetArgs([]string{
				"--execution-state-dir", execdir,
				"--output-dir", outdir,
				"--state-commitment", hex.EncodeToString(stateCommitment[:]),
				"--no-migration",
				"--no-report",
				"--output-payload-filename", outputPayloadFileName,
				"--extract-payloads-by-address", strings.Join(addresses, ","),
				"--chain", flow.Emulator.Chain().String()})

			err = Cmd.Execute()
			require.NoError(t, err)

			// Verify exported payloads.
			partialState, payloadsFromFile, err := util.ReadPayloadFile(zerolog.Nop(), outputPayloadFileName)
			require.NoError(t, err)
			require.Equal(t, len(selectedKeysValues), len(payloadsFromFile))
			require.True(t, partialState)

			for _, payloadFromFile := range payloadsFromFile {
				k, err := payloadFromFile.Key()
				require.NoError(t, err)

				kv, exist := selectedKeysValues[k.String()]
				require.True(t, exist)
				require.Equal(t, kv.value, payloadFromFile.Value())
			}
		})
	})
}

// TestExtractStateFromPayloads tests state extraction with payload as input.
func TestExtractStateFromPayloads(t *testing.T) {

	const payloadFileName = "root.payload"

	t.Run("create checkpoint", func(t *testing.T) {
		withDirs(t, func(_, execdir, outdir string) {
			size := 10

			inputPayloadFileName := filepath.Join(execdir, payloadFileName)

			// Generate some data
			keysValues := make(map[string]keyPair)
			var payloads []*ledger.Payload

			for i := 0; i < size; i++ {
				keys, values := getSampleKeyValues(i)

				for j, key := range keys {
					keysValues[key.String()] = keyPair{
						key:   key,
						value: values[j],
					}

					payloads = append(payloads, ledger.NewPayload(key, values[j]))
				}
			}

			numOfPayloadWritten, err := util.CreatePayloadFile(
				zerolog.Nop(),
				inputPayloadFileName,
				payloads,
				nil,
				false,
			)
			require.NoError(t, err)
			require.Equal(t, len(payloads), numOfPayloadWritten)

			// Export checkpoint file
			Cmd.SetArgs([]string{
				"--execution-state-dir", execdir,
				"--output-dir", outdir,
				"--no-migration",
				"--no-report",
				"--state-commitment", "",
				"--input-payload-filename", inputPayloadFileName,
				"--output-payload-filename", "",
				"--extract-payloads-by-address", "",
				"--chain", flow.Emulator.Chain().String()})

			err = Cmd.Execute()
			require.NoError(t, err)

			tries, err := wal.OpenAndReadCheckpointV6(outdir, "root.checkpoint", zerolog.Nop())
			require.NoError(t, err)
			require.Equal(t, 1, len(tries))

			// Verify exported checkpoint
			payloadsFromFile := tries[0].AllPayloads()
			require.NoError(t, err)
			require.Equal(t, len(keysValues), len(payloadsFromFile))

			for _, payloadFromFile := range payloadsFromFile {
				k, err := payloadFromFile.Key()
				require.NoError(t, err)

				kv, exist := keysValues[k.String()]
				require.True(t, exist)

				require.Equal(t, kv.value, payloadFromFile.Value())
			}
		})

	})

	t.Run("create payloads", func(t *testing.T) {
		withDirs(t, func(_, execdir, outdir string) {
			inputPayloadFileName := filepath.Join(execdir, payloadFileName)
			outputPayloadFileName := filepath.Join(outdir, "selected.payload")

			size := 10

			// Generate some data
			keysValues := make(map[string]keyPair)
			var payloads []*ledger.Payload

			for i := 0; i < size; i++ {
				keys, values := getSampleKeyValues(i)

				for j, key := range keys {
					keysValues[key.String()] = keyPair{
						key:   key,
						value: values[j],
					}

					payloads = append(payloads, ledger.NewPayload(key, values[j]))
				}
			}

			numOfPayloadWritten, err := util.CreatePayloadFile(
				zerolog.Nop(),
				inputPayloadFileName,
				payloads,
				nil,
				false,
			)
			require.NoError(t, err)
			require.Equal(t, len(payloads), numOfPayloadWritten)

			// Export all payloads
			Cmd.SetArgs([]string{
				"--execution-state-dir", execdir,
				"--output-dir", outdir,
				"--no-migration",
				"--no-report",
				"--state-commitment", "",
				"--input-payload-filename", inputPayloadFileName,
				"--output-payload-filename", outputPayloadFileName,
				"--extract-payloads-by-address", "",
				"--chain", flow.Emulator.Chain().String()})

			err = Cmd.Execute()
			require.NoError(t, err)

			// Verify exported payloads.
			partialState, payloadsFromFile, err := util.ReadPayloadFile(zerolog.Nop(), outputPayloadFileName)
			require.NoError(t, err)
			require.Equal(t, len(keysValues), len(payloadsFromFile))
			require.False(t, partialState)

			for _, payloadFromFile := range payloadsFromFile {
				k, err := payloadFromFile.Key()
				require.NoError(t, err)

				kv, exist := keysValues[k.String()]
				require.True(t, exist)

				require.Equal(t, kv.value, payloadFromFile.Value())
			}
		})
	})

	t.Run("input is partial state", func(t *testing.T) {
		withDirs(t, func(_, execdir, outdir string) {
			size := 10

			inputPayloadFileName := filepath.Join(execdir, payloadFileName)
			outputPayloadFileName := filepath.Join(outdir, "selected.payload")

			// Generate some data
			keysValues := make(map[string]keyPair)
			var payloads []*ledger.Payload

			for i := 0; i < size; i++ {
				keys, values := getSampleKeyValues(i)

				for j, key := range keys {
					keysValues[key.String()] = keyPair{
						key:   key,
						value: values[j],
					}

					payloads = append(payloads, ledger.NewPayload(key, values[j]))
				}
			}

			// Create input payload file that represents partial state
			numOfPayloadWritten, err := util.CreatePayloadFile(
				zerolog.Nop(),
				inputPayloadFileName,
				payloads,
				nil,
				true,
			)
			require.NoError(t, err)
			require.Equal(t, len(payloads), numOfPayloadWritten)

			// Since input payload file is partial state, --allow-partial-state-from-payload-file must be specified.
			Cmd.SetArgs([]string{
				"--execution-state-dir", execdir,
				"--output-dir", outdir,
				"--no-migration",
				"--no-report",
				"--state-commitment", "",
				"--input-payload-filename", inputPayloadFileName,
				"--output-payload-filename", outputPayloadFileName,
				"--extract-payloads-by-address", "",
				"--allow-partial-state-from-payload-file",
				"--chain", flow.Emulator.Chain().String()})

			err = Cmd.Execute()
			require.NoError(t, err)

			// Verify exported payloads.
			partialState, payloadsFromFile, err := util.ReadPayloadFile(zerolog.Nop(), outputPayloadFileName)
			require.NoError(t, err)
			require.Equal(t, len(keysValues), len(payloadsFromFile))
			require.True(t, partialState)

			for _, payloadFromFile := range payloadsFromFile {
				k, err := payloadFromFile.Key()
				require.NoError(t, err)

				kv, exist := keysValues[k.String()]
				require.True(t, exist)

				require.Equal(t, kv.value, payloadFromFile.Value())
			}
		})
	})
}

func getSampleKeyValues(i int) ([]ledger.Key, []ledger.Value) {
	switch i {
	case 0:
		return []ledger.Key{getKey("", "uuid"), getKey("", "account_address_state")},
			[]ledger.Value{[]byte{'1'}, []byte{'A'}}
	case 1:
		return []ledger.Key{getKey("ADDRESS", "public_key_count"),
				getKey("ADDRESS", "public_key_0"),
				getKey("ADDRESS", "exists"),
				getKey("ADDRESS", "storage_used")},
			[]ledger.Value{[]byte{1}, []byte("PUBLICKEYXYZ"), []byte{1}, []byte{100}}
	case 2:
		// TODO change the contract_names to CBOR encoding
		return []ledger.Key{getKey("ADDRESS", "contract_names"), getKey("ADDRESS", "code.mycontract")},
			[]ledger.Value{[]byte("mycontract"), []byte("CONTRACT Content")}
	default:
		keys := make([]ledger.Key, 0)
		values := make([]ledger.Value, 0)
		for j := 0; j < 10; j++ {
			// address := make([]byte, 32)
			address := make([]byte, 8)
			_, err := rand.Read(address)
			if err != nil {
				panic(err)
			}
			keys = append(keys, getKey(string(address), "test"))
			values = append(values, getRandomCadenceValue())
		}
		return keys, values
	}
}

func getKey(owner, key string) ledger.Key {
	return ledger.Key{KeyParts: []ledger.KeyPart{
		{Type: uint16(0), Value: []byte(owner)},
		{Type: uint16(2), Value: []byte(key)},
	},
	}
}

func getRandomCadenceValue() ledger.Value {

	randomPart := make([]byte, 10)
	_, err := rand.Read(randomPart)
	if err != nil {
		panic(err)
	}
	valueBytes := []byte{
		// magic prefix
		0x0, 0xca, 0xde, 0x0, 0x4,
		// tag
		0xd8, 132,
		// array, 5 items follow
		0x85,

		// tag
		0xd8, 193,
		// UTF-8 string, length 4
		0x64,
		// t, e, s, t
		0x74, 0x65, 0x73, 0x74,

		// nil
		0xf6,

		// positive integer 1
		0x1,

		// array, 0 items follow
		0x80,

		// UTF-8 string, length 10
		0x6a,
		0x54, 0x65, 0x73, 0x74, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74,
	}

	valueBytes = append(valueBytes, randomPart...)
	return ledger.Value(valueBytes)
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
