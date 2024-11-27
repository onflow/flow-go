package verify

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"
	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/fvm/evm/offchain/utils"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// Verify verifies the offchain replay of EVM blocks from the given height range
// and updates the EVM state gob files with the latest state
func Verify(
	log zerolog.Logger,
	from uint64,
	to uint64,
	chainID flow.ChainID,
	dataDir string,
	executionDataDir string,
	evmStateGobDir string,
	saveEveryNBlocks uint64,
) error {
	lg := log.With().
		Uint64("from", from).Uint64("to", to).
		Str("chain", chainID.String()).
		Str("dataDir", dataDir).
		Str("executionDataDir", executionDataDir).
		Str("evmStateGobDir", evmStateGobDir).
		Uint64("saveEveryNBlocks", saveEveryNBlocks).
		Logger()

	lg.Info().Msgf("verifying range from %d to %d", from, to)

	db, storages, executionDataStore, dsStore, err := initStorages(dataDir, executionDataDir)
	if err != nil {
		return fmt.Errorf("could not initialize storages: %w", err)
	}

	defer db.Close()
	defer dsStore.Close()

	var store *testutils.TestValueStore

	// root block require the account status registers to be saved
	isRoot := utils.IsEVMRootHeight(chainID, from)
	if isRoot {
		store = testutils.GetSimpleValueStore()
	} else {
		prev := from - 1
		store, err = loadState(prev, evmStateGobDir)
		if err != nil {
			return fmt.Errorf("could not load EVM state from previous height %d: %w", prev, err)
		}
	}

	// save state every N blocks
	onHeightReplayed := func(height uint64) error {
		log.Info().Msgf("replayed height %d", height)
		if height%saveEveryNBlocks == 0 {
			err := saveState(store, height, evmStateGobDir)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// replay blocks
	err = utils.OffchainReplayBackwardCompatibilityTest(
		log,
		chainID,
		from,
		to,
		storages.Headers,
		storages.Results,
		executionDataStore,
		store,
		onHeightReplayed,
	)

	if err != nil {
		return err
	}

	err = saveState(store, to, evmStateGobDir)
	if err != nil {
		return err
	}

	lg.Info().Msgf("successfully verified range from %d to %d", from, to)

	return nil
}

func saveState(store *testutils.TestValueStore, height uint64, gobDir string) error {
	valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(gobDir, height)
	values, allocators := store.Dump()
	err := testutils.SerializeState(valueFileName, values)
	if err != nil {
		return err
	}
	err = testutils.SerializeAllocator(allocatorFileName, allocators)
	if err != nil {
		return err
	}

	log.Info().Msgf("saved EVM state to %s and %s", valueFileName, allocatorFileName)

	return nil
}

func loadState(height uint64, gobDir string) (*testutils.TestValueStore, error) {
	valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(gobDir, height)
	values, err := testutils.DeserializeState(valueFileName)
	if err != nil {
		return nil, fmt.Errorf("could not deserialize state %v: %w", valueFileName, err)
	}

	allocators, err := testutils.DeserializeAllocator(allocatorFileName)
	if err != nil {
		return nil, fmt.Errorf("could not deserialize allocator %v: %w", allocatorFileName, err)
	}
	store := testutils.GetSimpleValueStorePopulated(values, allocators)

	log.Info().Msgf("loaded EVM state for height %d from gob file %v", height, valueFileName)
	return store, nil
}

func initStorages(dataDir string, executionDataDir string) (
	*badger.DB,
	*storage.All,
	execution_data.ExecutionDataGetter,
	io.Closer,
	error,
) {
	db := common.InitStorage(dataDir)

	storages := common.InitStorages(db)

	datastoreDir := filepath.Join(executionDataDir, "blobstore")
	err := os.MkdirAll(datastoreDir, 0700)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	dsOpts := &badgerds.DefaultOptions
	ds, err := badgerds.NewDatastore(datastoreDir, dsOpts)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	executionDataBlobstore := blobs.NewBlobstore(ds)
	executionDataStore := execution_data.NewExecutionDataStore(executionDataBlobstore, execution_data.DefaultSerializer)

	return db, storages, executionDataStore, ds, nil
}

func evmStateGobFileNamesByEndHeight(evmStateGobDir string, endHeight uint64) (string, string) {
	valueFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("values-%d.gob", endHeight))
	allocatorFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("allocators-%d.gob", endHeight))
	return valueFileName, allocatorFileName
}
