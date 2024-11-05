package re

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	ledgercomplete "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	exedataprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	edstorage "github.com/onflow/flow-go/module/executiondatasync/storage"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

var (
	flagExecutionStateDir string
	flagDatadir           string
	flagChunkDataPackDir  string
	flagBootstrapDir      string
	flagExecutionDataDir  string
	flagNodeID            string
	flagFrom              int
)

var Cmd = &cobra.Command{
	Use:   "re-execute-block",
	Short: "re-execute blocks",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "/var/flow/data/execution",
		"directory to the execution state")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.PersistentFlags().StringVarP(&flagDatadir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = Cmd.MarkPersistentFlagRequired("datadir")

	Cmd.PersistentFlags().StringVarP(&flagChunkDataPackDir, "chunk-data-pack-dir", "", "/var/flow/data/chunk-data-pack", "directory to the chunk data pack dababase")
	_ = Cmd.MarkPersistentFlagRequired("chunk-data-pack-dir")

	Cmd.PersistentFlags().StringVarP(&flagBootstrapDir, "bootstrap-dir", "", "/var/flow/bootstrap", "directory to the bootstrap folder")
	_ = Cmd.MarkPersistentFlagRequired("bootstrap-dir")

	Cmd.PersistentFlags().StringVarP(&flagExecutionDataDir, "execution-data-dir", "", "/var/flow/data/execution-data", "directory to the execution data folder")
	_ = Cmd.MarkPersistentFlagRequired("execution-data-dir")

	Cmd.PersistentFlags().StringVarP(&flagNodeID, "nodeid", "", "", "node id")
	_ = Cmd.MarkPersistentFlagRequired("nodeid")

	Cmd.Flags().IntVar(&flagFrom, "from", 0, "from segment")
	_ = Cmd.MarkPersistentFlagRequired("from")
}

func run(*cobra.Command, []string) {
	err := runWithFlags(flagDatadir, flagChunkDataPackDir, flagExecutionStateDir, flagBootstrapDir, flagExecutionDataDir, flagNodeID, uint64(flagFrom))
	if err != nil {
		log.Fatal().Err(err).Msg("could not run with flags")
	}
}

func runWithFlags(
	datadir string,
	chunkDataPackDir string,
	trieDir string,
	bootstrapDir string,
	executionDataDir string,
	nodeID string,
	height uint64,
) error {
	log.Info().
		Str("datadir", datadir).
		Str("chunkDataPackDir", chunkDataPackDir).
		Str("trieDir", trieDir).
		Str("bootstrapDir", bootstrapDir).
		Str("executionDataDir", executionDataDir).
		Str("nodeID", nodeID).
		Uint64("height", height).
		Msg("re-execute block")

	db := common.InitStorage(flagDatadir)
	defer db.Close()
	storages := common.InitStorages(db)

	mTrieCacheSize := 500
	diskWAL, err := wal.NewDiskWAL(log.With().Str("subcomponent", "wal").Logger(),
		nil, metrics.NewNoopCollector(), trieDir, mTrieCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return fmt.Errorf("failed to initialize wal: %w", err)
	}

	ledgerStorage, err := ledgercomplete.NewLedger(diskWAL, mTrieCacheSize, metrics.NewNoopCollector(), log.With().Str("subcomponent",
		"ledger").Logger(), ledgercomplete.DefaultPathFinderVersion)

	if err != nil {
		return fmt.Errorf("failed to initialize ledger: %w", err)
	}

	compactor, err := complete.NewCompactor(
		ledgerStorage, diskWAL,
		log.With().Str("subcomponent", "checkpointer").Logger(),
		500,
		100,
		5,
		atomic.NewBool(false),
		metrics.NewNoopCollector(),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize compactor: %w", err)
	}
	<-compactor.Ready()
	<-ledgerStorage.Ready()

	execState, err := createExecutionState(chunkDataPackDir, ledgerStorage, storages, db)
	if err != nil {
		return fmt.Errorf("failed to create execution state: %w", err)
	}

	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		return err
	}

	myID, err := flow.HexStringToIdentifier(nodeID)
	if err != nil {
		return fmt.Errorf("could not parse node ID from string (id: %v): %w", nodeID, err)
	}

	self, err := state.Final().Identity(myID)
	if err != nil {
		return fmt.Errorf("node identity not found in the identity list of the finalized state (id: %v): %w", myID, err)
	}

	info, err := cmd.LoadPrivateNodeInfo(bootstrapDir, myID)
	if err != nil {
		return fmt.Errorf("failed to load private node info: %w", err)
	}

	me, err := local.New(self.IdentitySkeleton, info.StakingPrivKey.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create local node: %w", err)
	}

	opts := initFvmOptions(storages.Headers, state.Params().ChainID())
	vmCtx := fvm.NewContext(opts...)

	ledgerViewCommitter := committer.NewLedgerViewCommitter(ledgerStorage, trace.NewNoopTracer())

	sealed, err := state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("cannot get the sealed block: %w", err)
	}

	trackerDir := filepath.Join(executionDataDir, "tracker")
	executionDataTracker, err := tracker.OpenStorage(
		trackerDir,
		sealed.Height,
		log.Logger,
		tracker.WithPruneCallback(func(c cid.Cid) error {
			return nil
		}),
	)
	if err != nil {
		return err
	}

	datastoreDir := filepath.Join(executionDataDir, "blobstore")
	edm, err := edstorage.NewBadgerDatastoreManager(datastoreDir, &badgerds.DefaultOptions)
	if err != nil {
		return err
	}
	ds := edm.Datastore()
	blobService := newLocalBlobService(ds)

	executionDataProvider := exedataprovider.NewProvider(
		log.Logger,
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		blobService,
		executionDataTracker,
	)

	computationManager, err := computation.New(
		log.Logger,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		me,
		state,
		vmCtx,
		ledgerViewCommitter,
		executionDataProvider,
		computation.ComputationConfig{
			QueryConfig:          query.NewDefaultConfig(),
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)

	if err != nil {
		return err
	}

	err = ExecuteBlock(execState, computationManager, storages.Headers, storages.Blocks, storages.Commits, storages.Collections,
		height)
	if err != nil {
		log.Fatal().Err(err).Msg("could not execute block")
	}

	return nil
}

func ExecuteBlock(
	execState state.ExecutionState,
	computationManager computation.ComputationManager,
	headers storage.Headers,
	blocks storage.Blocks,
	commits storage.Commits,
	collections storage.Collections,
	height uint64,
) error {
	block, err := readBlock(headers, blocks, commits, collections, height)
	if err != nil {
		return err
	}

	log.Info().Msgf("executing block %v", block.Block.Header.ID())

	result, err := executeBlock(execState, computationManager, context.Background(), block)
	if err != nil {
		return err
	}

	log.Info().Msgf("block %v executed, result ID: %v", block.Block.Header.ID(), result.ID())

	return nil
}

func executeBlock(
	execState state.ExecutionState,
	computationManager computation.ComputationManager,
	ctx context.Context,
	executableBlock *entity.ExecutableBlock,
) (*execution.ComputationResult, error) {
	parentID := executableBlock.Block.Header.ParentID
	parentErID, err := execState.GetExecutionResultID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent execution result ID %v: %w", parentID, err)
	}

	snapshot := execState.NewStorageSnapshot(*executableBlock.StartState,
		executableBlock.Block.Header.ParentID,
		executableBlock.Block.Header.Height-1,
	)

	log.Info().Msgf("computing block %v", executableBlock.Block.Header.ID())

	computationResult, err := computationManager.ComputeBlock(
		ctx,
		parentErID,
		executableBlock,
		snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to compute block: %w", err)
	}

	return computationResult, nil
}

func readBlock(
	headers storage.Headers,
	blocks storage.Blocks,
	commits storage.Commits,
	collections storage.Collections,
	height uint64,
) (*entity.ExecutableBlock, error) {
	blockID, err := headers.BlockIDByHeight(height)
	if err != nil {
		return nil, err
	}

	block, err := blocks.ByID(blockID)
	if err != nil {
		return nil, err
	}

	startCommit, err := commits.ByBlockID(block.Header.ParentID)
	if err != nil {
		return nil, err
	}

	cols := make(map[flow.Identifier]*entity.CompleteCollection, len(block.Payload.Guarantees))
	for _, cg := range block.Payload.Guarantees {
		col, err := collections.ByID(cg.CollectionID)
		if err != nil {
			return nil, err
		}
		cols[cg.CollectionID] = &entity.CompleteCollection{
			Guarantee:    cg,
			Transactions: col.Transactions,
		}
	}

	return &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: cols,
		StartState:          &startCommit,
		Executing:           false,
	}, nil
}

func createExecutionState(chunkDataPackDir string, ledgerStorage ledger.Ledger, storages *storage.All, db *badger.DB) (state.ExecutionState, error) {

	myReceipts := badgerstorage.NewMyExecutionReceipts(metrics.NewNoopCollector(), db, storages.Receipts.(*badgerstorage.ExecutionReceipts))
	serviceEvents := badgerstorage.NewServiceEvents(metrics.NewNoopCollector(), db)

	chunkDataPackDB, err := pebblestorage.OpenDefaultPebbleDB(chunkDataPackDir)
	if err != nil {
		return nil, err
	}
	chunkDataPacks := pebblestorage.NewChunkDataPacks(metrics.NewNoopCollector(),
		chunkDataPackDB, storages.Collections, 1000)

	return state.NewExecutionState(
		ledgerStorage,
		storages.Commits,
		storages.Blocks,
		storages.Headers,
		storages.Collections,
		chunkDataPacks,
		storages.Results,
		myReceipts,
		storages.Events,
		serviceEvents,
		storages.TransactionResults,
		db,
		trace.NewNoopTracer(),
		nil,
		false,
	), nil

}

func initFvmOptions(headers storage.Headers, chainID flow.ChainID) []fvm.Option {
	blockFinder := environment.NewBlockFinder(headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(chainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
	}
	switch chainID {
	case flow.Testnet,
		flow.Sandboxnet,
		flow.Previewnet,
		flow.Mainnet:
		vmOpts = append(vmOpts,
			fvm.WithTransactionFeesEnabled(true),
		)
	}
	switch chainID {
	case flow.Testnet,
		flow.Sandboxnet,
		flow.Previewnet,
		flow.Localnet,
		flow.Benchnet:
		vmOpts = append(vmOpts,
			fvm.WithContractDeploymentRestricted(false),
		)
	}
	return vmOpts
}

type blobService struct {
	component.Component
	blockService blockservice.BlockService
	blockStore   blockstore.Blockstore
}

func newLocalBlobService(
	ds datastore.Batching,
) *blobService {
	blockStore := blockstore.NewBlockstore(ds)
	bs := &blobService{
		blockService: blockservice.New(blockStore, nil),
		blockStore:   blockStore,
	}
	return bs
}

func (bs *blobService) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	return bs.blockService.GetBlock(ctx, c)
}

func (bs *blobService) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return bs.blockService.GetBlocks(ctx, ks)
}

func (bs *blobService) AddBlob(ctx context.Context, b blobs.Blob) error {
	return bs.blockService.AddBlock(ctx, b)
}

func (bs *blobService) AddBlobs(ctx context.Context, blobs []blobs.Blob) error {
	return bs.blockService.AddBlocks(ctx, blobs)
}

func (bs *blobService) DeleteBlob(ctx context.Context, c cid.Cid) error {
	return bs.blockService.DeleteBlock(ctx, c)
}

func (bs *blobService) GetSession(ctx context.Context) network.BlobGetter {
	return nil
}

func (bs *blobService) TriggerReprovide(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}
