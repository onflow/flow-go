package verify

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"
	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/offchain/utils"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// Verify verifies the offchain replay of EVM blocks from the given height range
// and updates the EVM state gob files with the latest state
func Verify(log zerolog.Logger, from uint64, to uint64, chainID flow.ChainID, dataDir string, executionDataDir string, evmStateGobDir string) error {
	log.Info().
		Str("chain", chainID.String()).
		Str("dataDir", dataDir).
		Str("executionDataDir", executionDataDir).
		Str("evmStateGobDir", evmStateGobDir).
		Msgf("verifying range from %d to %d", from, to)

	db, storages, executionDataStore, dsStore, err := initStorages(chainID, dataDir, executionDataDir)
	if err != nil {
		return fmt.Errorf("could not initialize storages: %w", err)
	}

	defer db.Close()
	defer dsStore.Close()

	var store *testutils.TestValueStore
	isRoot := isEVMRootHeight(chainID, from)
	if isRoot {
		log.Info().Msgf("initializing EVM state for root height %d", from)

		store = testutils.GetSimpleValueStore()
		as := environment.NewAccountStatus()
		rootAddr := evm.StorageAccountAddress(chainID)
		err = store.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
		if err != nil {
			return err
		}
	} else {
		prev := from - 1
		log.Info().Msgf("loading EVM state from previous height %d", prev)

		valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(evmStateGobDir, prev)
		values, err := testutils.DeserializeState(valueFileName)
		if err != nil {
			return fmt.Errorf("could not deserialize state %v: %w", valueFileName, err)
		}

		allocators, err := testutils.DeserializeAllocator(allocatorFileName)
		if err != nil {
			return fmt.Errorf("could not deserialize allocator %v: %w", allocatorFileName, err)
		}
		store = testutils.GetSimpleValueStorePopulated(values, allocators)
	}

	err = utils.OffchainReplayBackwardCompatibilityTest(
		log,
		chainID,
		from,
		to,
		storages.Headers,
		storages.Results,
		executionDataStore,
		store,
	)

	if err != nil {
		return err
	}

	valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(evmStateGobDir, to)
	values, allocators := store.Dump()
	err = testutils.SerializeState(valueFileName, values)
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

func initStorages(chainID flow.ChainID, dataDir string, executionDataDir string) (
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

// EVM Root Height is the first block that has EVM Block Event where the EVM block height is 1
func isEVMRootHeight(chainID flow.ChainID, flowHeight uint64) bool {
	if chainID == flow.Testnet {
		return flowHeight == 211176671
	} else if chainID == flow.Mainnet {
		return flowHeight == 85981136
	}
	return flowHeight == 1
}

func evmStateGobFileNamesByEndHeight(evmStateGobDir string, endHeight uint64) (string, string) {
	valueFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("values-%d.gob", endHeight))
	allocatorFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("allocators-%d.gob", endHeight))
	return valueFileName, allocatorFileName
}
