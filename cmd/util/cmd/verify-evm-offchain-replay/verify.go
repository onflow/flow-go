package verify

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"
	badgerds "github.com/ipfs/go-ds-badger2"

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

func Verify(from uint64, to uint64, chainID flow.ChainID, dataDir string, executionDataDir string, evmStateGobDir string) error {
	db, storages, executionDataStore, dsStore, err := initStorages(chainID, dataDir, executionDataDir)
	if err != nil {
		return fmt.Errorf("could not initialize storages: %w", err)
	}

	defer db.Close()
	defer dsStore.Close()

	var store *testutils.TestValueStore
	isRoot := isEVMRootHeight(chainID, from)
	if isRoot {
		store = testutils.GetSimpleValueStore()
		as := environment.NewAccountStatus()
		rootAddr := evm.StorageAccountAddress(chainID)
		err = store.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
		if err != nil {
			return err
		}
	}

	return utils.OffchainReplayBackwardCompatibilityTest(
		chainID,
		from,
		to,
		storages.Headers,
		storages.Results,
		executionDataStore,
		store,
	)
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
