package utils_test

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/offchain/utils"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func TestTestnetBackwardCompatibility(t *testing.T) {
	t.Skip("TIME CONSUMING TESTS. Enable the tests with the events files saved in local")
	// how to run this tests
	// Note: this is a time consuming tests, so please run it in local
	//
	// 1) run the following cli to get the events files across different sporks

	// flow events get A.8c5303eaa26202d6.EVM.TransactionExecuted A.8c5303eaa26202d6.EVM.BlockExecuted
	//   --start 211176670 --end 211176770 --network testnet --host access-001.devnet51.nodes.onflow.org:9000
	// > ~/Downloads/events_devnet51_1.jsonl
	// ...
	//
	// 2) comment the above t.Skip, and update the events file paths and evmStateGob dir
	// to run the tests
	BackwardCompatibleSinceEVMGenesisBlock(
		t, flow.Testnet, []string{
			"~/Downloads/events_devnet51_1.jsonl",
			"~/Downloads/events_devnet51_2.jsonl",
		},
		"~/Downloads/",
		0,
	)
}

// BackwardCompatibilityTestSinceEVMGenesisBlock ensures that the offchain package
// can read EVM events from the provided file paths, replay blocks starting from
// the EVM genesis block, and derive a consistent state matching the latest on-chain EVM state.
//
// The parameter `eventsFilePaths` is a list of file paths containing ordered EVM events in JSONL format.
// These EVM event files can be generated using the Flow CLI query command, for example:
//
// flow events get A.8c5303eaa26202d6.EVM.TransactionExecuted A.8c5303eaa26202d6.EVM.BlockExecuted
//
//	--start 211176670 --end 211176770 --network testnet --host access-001.devnet51.nodes.onflow.org:9000
//
// During the replay process, it will generate `values_<height>.gob` and
// `allocators_<height>.gob` checkpoint files for each height. If these checkpoint gob files exist,
// the corresponding event JSON files will be skipped to optimize replay.
func BackwardCompatibleSinceEVMGenesisBlock(
	t *testing.T,
	chainID flow.ChainID,
	eventsFilePaths []string, // ordered EVM events in JSONL format
	evmStateGob string,
	evmStateEndHeight uint64, // EVM height of an EVM state that a evmStateGob file was created for
) {
	// ensure that event files is not an empty array
	require.True(t, len(eventsFilePaths) > 0)

	log.Info().Msgf("replaying EVM events from %v to %v, with evmStateGob file in %s, and evmStateEndHeight: %v",
		eventsFilePaths[0], eventsFilePaths[len(eventsFilePaths)-1],
		evmStateGob, evmStateEndHeight)

	store, evmStateEndHeightOrZero := initStorageWithEVMStateGob(t, chainID, evmStateGob, evmStateEndHeight)

	// the events to replay
	nextHeight := evmStateEndHeightOrZero + 1

	// replay each event files
	for _, eventsFilePath := range eventsFilePaths {
		log.Info().Msgf("replaying events from %v, nextHeight: %v", eventsFilePath, nextHeight)

		evmStateEndHeight := replayEvents(t, chainID, store, eventsFilePath, evmStateGob, nextHeight)
		nextHeight = evmStateEndHeight + 1
	}

	log.Info().
		Msgf("succhessfully replayed all events and state changes are consistent with onchain state change. nextHeight: %v", nextHeight)
}

func initStorageWithEVMStateGob(t *testing.T, chainID flow.ChainID, evmStateGob string, evmStateEndHeight uint64) (
	*TestValueStore, uint64,
) {
	rootAddr := evm.StorageAccountAddress(chainID)

	// if there is no evmStateGob file, create a empty store and initialize the account status,
	// return 0 as the genesis height
	if evmStateEndHeight == 0 {
		store := GetSimpleValueStore()
		as := environment.NewAccountStatus()
		require.NoError(t, store.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes()))

		return store, 0
	}

	valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(evmStateGob, evmStateEndHeight)
	values, err := DeserializeState(valueFileName)
	require.NoError(t, err)
	allocators, err := DeserializeAllocator(allocatorFileName)
	require.NoError(t, err)
	store := GetSimpleValueStorePopulated(values, allocators)
	return store, evmStateEndHeight
}

func replayEvents(
	t *testing.T,
	chainID flow.ChainID,
	store *TestValueStore, eventsFilePath string, evmStateGob string, initialNextHeight uint64) uint64 {

	rootAddr := evm.StorageAccountAddress(chainID)

	bpStorage := storage.NewEphemeralStorage(store)
	bp, err := blocks.NewBasicProvider(chainID, bpStorage, rootAddr)
	require.NoError(t, err)

	nextHeight := initialNextHeight

	scanEventFilesAndRun(t, eventsFilePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
			if blockEventPayload.Height != nextHeight {
				return fmt.Errorf(
					"expected height for next block event to be %v, but got %v",
					nextHeight, blockEventPayload.Height)
			}

			err = bp.OnBlockReceived(blockEventPayload)
			require.NoError(t, err)

			sp := NewTestStorageProvider(store, blockEventPayload.Height)
			cr := sync.NewReplayer(chainID, rootAddr, sp, bp, zerolog.Logger{}, nil, true)
			res, results, err := cr.ReplayBlock(txEvents, blockEventPayload)
			require.NoError(t, err)

			// commit all changes
			for k, v := range res.StorageRegisterUpdates() {
				err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			proposal := blocks.ReconstructProposal(blockEventPayload, txEvents, results)

			err = bp.OnBlockExecuted(blockEventPayload.Height, res, proposal)
			require.NoError(t, err)

			// commit all block hash list changes
			for k, v := range bpStorage.StorageRegisterUpdates() {
				err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			// verify the block height is sequential without gap
			nextHeight++

			return nil
		})

	evmStateEndHeight := nextHeight - 1

	log.Info().Msgf("finished replaying events from %v to %v, creating evm state gobs", initialNextHeight, evmStateEndHeight)
	valuesFile, allocatorsFile := dumpEVMStateToGobFiles(t, store, evmStateGob, evmStateEndHeight)
	log.Info().Msgf("evm state gobs created: %v, %v", valuesFile, allocatorsFile)

	return evmStateEndHeight
}

func evmStateGobFileNamesByEndHeight(dir string, endHeight uint64) (string, string) {
	return filepath.Join(dir, fmt.Sprintf("values_%d.gob", endHeight)),
		filepath.Join(dir, fmt.Sprintf("allocators_%d.gob", endHeight))
}

func dumpEVMStateToGobFiles(t *testing.T, store *TestValueStore, dir string, evmStateEndHeight uint64) (string, string) {
	valuesFileName, allocatorsFileName := evmStateGobFileNamesByEndHeight(dir, evmStateEndHeight)
	values, allocators := store.Dump()

	require.NoError(t, SerializeState(valuesFileName, values))
	require.NoError(t, SerializeAllocator(allocatorsFileName, allocators))
	return valuesFileName, allocatorsFileName
}

// scanEventFilesAndRun
func scanEventFilesAndRun(
	t *testing.T,
	filePath string,
	handler func(*events.BlockEventPayload, []events.TransactionEventPayload) error,
) {
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	txEvents := make([]events.TransactionEventPayload, 0)

	for scanner.Scan() {
		data := scanner.Bytes()
		var e utils.Event
		err := json.Unmarshal(data, &e)
		require.NoError(t, err)
		if strings.Contains(e.EventType, "BlockExecuted") {
			temp, err := hex.DecodeString(e.EventPayload)
			require.NoError(t, err)
			ev, err := ccf.Decode(nil, temp)
			require.NoError(t, err)
			blockEventPayload, err := events.DecodeBlockEventPayload(ev.(cadence.Event))
			require.NoError(t, err)

			require.NoError(t, handler(blockEventPayload, txEvents), fmt.Sprintf("fail to handle block at height %d",
				blockEventPayload.Height))

			txEvents = make([]events.TransactionEventPayload, 0)
			continue
		}

		temp, err := hex.DecodeString(e.EventPayload)
		require.NoError(t, err)
		ev, err := ccf.Decode(nil, temp)
		require.NoError(t, err)
		txEv, err := events.DecodeTransactionEventPayload(ev.(cadence.Event))
		require.NoError(t, err)
		txEvents = append(txEvents, *txEv)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
