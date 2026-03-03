package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cockroachdb/pebble/v2"
	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/go-cid"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/admin/commands"
	executionCommands "github.com/onflow/flow-go/admin/commands/execution"
	stateSyncCommands "github.com/onflow/flow-go/admin/commands/state_synchronization"
	storageCommands "github.com/onflow/flow-go/admin/commands/storage"
	uploaderCommands "github.com/onflow/flow-go/admin/commands/uploader"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/execution/checker"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	txmetrics "github.com/onflow/flow-go/engine/execution/computation/metrics"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/fetcher"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	exeprovider "github.com/onflow/flow-go/engine/execution/provider"
	exepruner "github.com/onflow/flow-go/engine/execution/pruner"
	"github.com/onflow/flow-go/engine/execution/rpc"
	"github.com/onflow/flow-go/engine/execution/scripts"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	ledgerfactory "github.com/onflow/flow-go/ledger/factory"
	modelbootstrap "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	exedataprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/executiondatasync/pruner"
	edstorage "github.com/onflow/flow-go/module/executiondatasync/storage"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/underlay"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

const (
	blockDataUploaderMaxRetry     uint64 = 5
	blockdataUploaderRetryTimeout        = 1 * time.Second
)

type ExecutionNodeBuilder struct {
	*FlowNodeBuilder                  // the common configs as a node
	exeConf          *ExecutionConfig // the configs and flags specific for execution node
}

func NewExecutionNodeBuilder(nodeBuilder *FlowNodeBuilder) *ExecutionNodeBuilder {
	return &ExecutionNodeBuilder{
		FlowNodeBuilder: nodeBuilder,
		exeConf:         &ExecutionConfig{},
	}
}

func (builder *ExecutionNodeBuilder) LoadFlags() {
	builder.FlowNodeBuilder.
		ExtraFlags(builder.exeConf.SetupFlags).
		ValidateFlags(builder.exeConf.ValidateFlags)
}

// ExecutionNode contains the running modules and their loading code.
type ExecutionNode struct {
	builder *FlowNodeBuilder // This is needed for accessing the ShutdownFunc
	exeConf *ExecutionConfig

	ingestionUnit *engine.Unit

	collector      *metrics.ExecutionCollector
	executionState state.ExecutionState
	followerState  protocol.FollowerState
	committee      hotstuff.DynamicCommittee
	ledgerStorage  ledger.Ledger
	registerStore  *storehouse.RegisterStore

	// storage
	events          storageerr.Events
	eventsReader    storageerr.EventsReader
	serviceEvents   storageerr.ServiceEvents
	txResults       storageerr.TransactionResults
	txResultsReader storageerr.TransactionResultsReader
	results         storageerr.ExecutionResults
	resultsReader   storageerr.ExecutionResultsReader
	receipts        storageerr.ExecutionReceipts
	myReceipts      storageerr.MyExecutionReceipts
	commits         storageerr.Commits
	commitsReader   storageerr.CommitsReader
	collections     storageerr.Collections

	chunkDataPackDB        *pebble.DB
	chunkDataPacks         storageerr.ChunkDataPacks
	providerEngine         exeprovider.ProviderEngine
	checkerEng             *checker.Engine
	syncCore               *chainsync.Core
	syncEngine             *synchronization.Engine
	followerCore           *hotstuff.FollowerLoop        // follower hotstuff logic
	followerEng            *followereng.ComplianceEngine // to sync blocks from consensus nodes
	computationManager     *computation.Manager
	collectionRequester    ingestion.CollectionRequester
	scriptsEng             *scripts.Engine
	followerDistributor    *pubsub.FollowerDistributor
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error)
	blockDataUploader      *uploader.Manager
	executionDataStore     execution_data.ExecutionDataStore
	toTriggerCheckpoint    *atomic.Bool      // create the checkpoint trigger to be controlled by admin tool, and listened by the compactor
	stopControl            *stop.StopControl // stop the node at given block height
	executionDataDatastore edstorage.DatastoreManager
	executionDataPruner    *pruner.Pruner
	executionDataBlobstore blobs.Blobstore
	executionDataTracker   tracker.Storage
	blobService            network.BlobService
	blobserviceDependable  *module.ProxiedReadyDoneAware
	metricsProvider        txmetrics.TransactionExecutionMetricsProvider

	// used by ingestion engine to notify executed block, and
	// used by background indexer engine to trigger indexing
	blockExecutedNotifier *ingestion.BlockExecutedNotifier

	// save register updates in storehouse when it is not enabled
	backgroundIndexerEngine *storehouse.BackgroundIndexerEngine
}

func (builder *ExecutionNodeBuilder) LoadComponentsAndModules() {

	exeNode := &ExecutionNode{
		builder:             builder.FlowNodeBuilder,
		exeConf:             builder.exeConf,
		toTriggerCheckpoint: atomic.NewBool(false),
		ingestionUnit:       engine.NewUnit(),
	}

	builder.FlowNodeBuilder.
		AdminCommand("read-execution-data", func(config *NodeConfig) commands.AdminCommand {
			return stateSyncCommands.NewReadExecutionDataCommand(exeNode.executionDataStore)
		}).
		AdminCommand("trigger-checkpoint", func(config *NodeConfig) commands.AdminCommand {
			return executionCommands.NewTriggerCheckpointCommand(exeNode.toTriggerCheckpoint, exeNode.exeConf.ledgerServiceAddr, exeNode.exeConf.ledgerServiceAdminAddr)
		}).
		AdminCommand("stop-at-height", func(config *NodeConfig) commands.AdminCommand {
			return executionCommands.NewStopAtHeightCommand(exeNode.stopControl)
		}).
		AdminCommand("set-uploader-enabled", func(config *NodeConfig) commands.AdminCommand {
			return uploaderCommands.NewToggleUploaderCommand(exeNode.blockDataUploader)
		}).
		AdminCommand("protocol-snapshot", func(conf *NodeConfig) commands.AdminCommand {
			return storageCommands.NewProtocolSnapshotCommand(
				conf.Logger,
				conf.State,
				conf.Storage.Headers,
				conf.Storage.Seals,
				exeNode.exeConf.triedir,
			)
		}).
		Module("load collections", exeNode.LoadCollections).
		Module("mutable follower state", exeNode.LoadMutableFollowerState).
		Module("system specs", exeNode.LoadSystemSpecs).
		Module("execution metrics", exeNode.LoadExecutionMetrics).
		Module("sync core", exeNode.LoadSyncCore).
		Module("execution storage", exeNode.LoadExecutionStorage).
		Module("follower distributor", exeNode.LoadFollowerDistributor).
		Module("block executed notifier", exeNode.LoadBlockExecutedNotifier).
		Module("authorization checking function", exeNode.LoadAuthorizationCheckingFunction).
		Module("execution data datastore", exeNode.LoadExecutionDataDatastore).
		Module("execution data getter", exeNode.LoadExecutionDataGetter).
		Module("blobservice peer manager dependencies", exeNode.LoadBlobservicePeerManagerDependencies).
		Module("bootstrap", exeNode.LoadBootstrapper).
		Module("register store", exeNode.LoadRegisterStore).
		AdminCommand("get-transactions", func(conf *NodeConfig) commands.AdminCommand {
			return storageCommands.NewGetTransactionsCommand(conf.State, conf.Storage.Payloads, exeNode.collections)
		}).
		Component("execution state ledger", exeNode.LoadExecutionStateLedger).
		// TODO: Modules should be able to depends on components
		// Because all modules are always bootstrapped first, before components,
		// its not possible to have a module depending on a Component.
		// This is the case for a StopControl which needs to query ExecutionState which needs execution state ledger.
		// I prefer to use dummy component now and keep the bootstrapping steps properly separated,
		// so it will be easier to follow and refactor later
		Component("execution state", exeNode.LoadExecutionState).
		// Load the admin tool when chunk data packs db are initialized in execution state
		AdminCommand("create-chunk-data-packs-checkpoint", func(config *NodeConfig) commands.AdminCommand {
			// by default checkpoints will be created under "/data/chunk_data_packs_checkpoints_dir"
			return storageCommands.NewPebbleDBCheckpointCommand(exeNode.exeConf.chunkDataPackCheckpointsDir,
				"chunk_data_pack", exeNode.chunkDataPackDB)
		}).
		Component("stop control", exeNode.LoadStopControl).
		// disable execution data pruner for now, since storehouse is going to need the execution data
		// for recovery,
		// TODO: will re-visit this once storehouse has implemented new WAL for checkpoint file of
		// payloadless trie.
		// Component("execution data pruner", exeNode.LoadExecutionDataPruner).
		Component("execution db pruner", exeNode.LoadExecutionDBPruner).
		Component("blob service", exeNode.LoadBlobService).
		Component("block data upload manager", exeNode.LoadBlockUploaderManager).
		Component("GCP block data uploader", exeNode.LoadGCPBlockDataUploader).
		Component("S3 block data uploader", exeNode.LoadS3BlockDataUploader).
		Component("transaction execution metrics", exeNode.LoadTransactionExecutionMetrics).
		Component("provider engine", exeNode.LoadProviderEngine).
		Component("checker engine", exeNode.LoadCheckerEngine).
		Component("ingestion engine", exeNode.LoadIngestionEngine).
		Component("scripts engine", exeNode.LoadScriptsEngine).
		Component("consensus committee", exeNode.LoadConsensusCommittee).
		Component("follower core", exeNode.LoadFollowerCore).
		Component("follower engine", exeNode.LoadFollowerEngine).
		Component("collection requester engine", exeNode.LoadCollectionRequesterEngine).
		Component("receipt provider engine", exeNode.LoadReceiptProviderEngine).
		Component("synchronization engine", exeNode.LoadSynchronizationEngine).
		Component("grpc server", exeNode.LoadGrpcServer)

	// Only load background indexer engine when both flags indicate it should be enabled
	if !exeNode.exeConf.enableStorehouse && exeNode.exeConf.enableBackgroundStorehouseIndexing {
		builder.FlowNodeBuilder.Component("background indexer engine", exeNode.LoadBackgroundIndexerEngine)
	}
}

func (exeNode *ExecutionNode) LoadCollections(node *NodeConfig) error {
	transactions := store.NewTransactions(node.Metrics.Cache, node.ProtocolDB)
	exeNode.collections = store.NewCollections(node.ProtocolDB, transactions)
	return nil
}

func (exeNode *ExecutionNode) LoadMutableFollowerState(node *NodeConfig) error {
	// For now, we only support state implementations from package badger.
	// If we ever support different implementations, the following can be replaced by a type-aware factory
	bState, ok := node.State.(*badgerState.State)
	if !ok {
		return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
	}
	var err error
	exeNode.followerState, err = badgerState.NewFollowerState(
		node.Logger,
		node.Tracer,
		node.ProtocolEvents,
		bState,
		node.Storage.Index,
		node.Storage.Payloads,
		blocktimer.DefaultBlockTimer,
	)
	return err
}

func (exeNode *ExecutionNode) LoadSystemSpecs(node *NodeConfig) error {
	sysInfoLogger := node.Logger.With().Str("system", "specs").Logger()
	err := logSysInfo(sysInfoLogger)
	if err != nil {
		sysInfoLogger.Error().Err(err)
	}
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionMetrics(node *NodeConfig) error {
	exeNode.collector = metrics.NewExecutionCollector(node.Tracer)

	// report the highest executed block height as soon as possible
	// this is guaranteed to exist because LoadBootstrapper has inserted
	// the root block as executed block
	var blockID flow.Identifier

	err := operation.RetrieveExecutedBlock(node.ProtocolDB.Reader(), &blockID)
	if err != nil {
		// database has not been bootstrapped yet
		if errors.Is(err, storageerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("could not get highest executed block: %w", err)
	}

	executed, err := node.Storage.Headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get header by id: %v: %w", blockID, err)
	}

	exeNode.collector.ExecutionLastExecutedBlockHeight(executed.Height)
	return nil
}

func (exeNode *ExecutionNode) LoadSyncCore(node *NodeConfig) error {
	var err error
	exeNode.syncCore, err = chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector(node.RootChainID), node.RootChainID)
	return err
}

func (exeNode *ExecutionNode) LoadExecutionStorage(
	node *NodeConfig,
) error {
	var err error
	db := node.ProtocolDB

	exeNode.events = store.NewEvents(node.Metrics.Cache, db)
	exeNode.serviceEvents = store.NewServiceEvents(node.Metrics.Cache, db)
	exeNode.commits = store.NewCommits(node.Metrics.Cache, db)
	exeNode.results = store.NewExecutionResults(node.Metrics.Cache, db)
	exeNode.receipts = store.NewExecutionReceipts(node.Metrics.Cache, db, exeNode.results, storage.DefaultCacheSize)
	exeNode.myReceipts = store.NewMyExecutionReceipts(node.Metrics.Cache, db, exeNode.receipts)
	exeNode.txResults, err = store.NewTransactionResults(node.Metrics.Cache, db, exeNode.exeConf.transactionResultsCacheSize)
	if err != nil {
		return err
	}
	exeNode.eventsReader = exeNode.events
	exeNode.commitsReader = exeNode.commits
	exeNode.resultsReader = exeNode.results
	exeNode.txResultsReader = exeNode.txResults
	return nil
}

func (exeNode *ExecutionNode) LoadFollowerDistributor(node *NodeConfig) error {
	exeNode.followerDistributor = pubsub.NewFollowerDistributor()
	exeNode.followerDistributor.AddProposalViolationConsumer(notifications.NewSlashingViolationsConsumer(node.Logger))
	return nil
}

func (exeNode *ExecutionNode) LoadBlockExecutedNotifier(node *NodeConfig) error {
	// background storehouse indexing is the only consumer of this notifier,
	// only create the notifier when background storehouse indexing is enabled
	if !exeNode.exeConf.enableBackgroundStorehouseIndexing {
		return nil
	}

	exeNode.blockExecutedNotifier = ingestion.NewBlockExecutedNotifier()

	return nil
}

func (exeNode *ExecutionNode) LoadBlobService(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// build list of Access nodes that are allowed to request execution data from this node
	var allowedANs map[flow.Identifier]bool
	if exeNode.exeConf.executionDataAllowedPeers != "" {
		ids := strings.Split(exeNode.exeConf.executionDataAllowedPeers, ",")
		allowedANs = make(map[flow.Identifier]bool, len(ids))
		for _, idHex := range ids {
			anID, err := flow.HexStringToIdentifier(idHex)
			if err != nil {
				return nil, fmt.Errorf("invalid node ID %s: %w", idHex, err)
			}

			id, ok := exeNode.builder.IdentityProvider.ByNodeID(anID)
			if !ok {
				return nil, fmt.Errorf("allowed node ID %s is not in identity list", idHex)
			}

			if id.Role != flow.RoleAccess {
				return nil, fmt.Errorf("allowed node ID %s is not an access node", id.NodeID.String())
			}

			if id.IsEjected() {
				exeNode.builder.Logger.Warn().
					Str("node_id", idHex).
					Msg("removing Access Node from the set of nodes authorized to request Execution Data, because it is ejected")
				continue
			}

			allowedANs[anID] = true
		}
	}

	opts := []network.BlobServiceOption{
		blob.WithBitswapOptions(
			// Only allow block requests from staked ENs and ANs on the allowedANs list (if set)
			bitswap.WithPeerBlockRequestFilter(
				blob.AuthorizedRequester(allowedANs, exeNode.builder.IdentityProvider, exeNode.builder.Logger),
			),
			bitswap.WithTracer(
				blob.NewTracer(node.Logger.With().Str("blob_service", channels.ExecutionDataService.String()).Logger()),
			),
		),
	}

	if !node.BitswapReprovideEnabled {
		opts = append(opts, blob.WithReprovideInterval(-1))
	}

	if exeNode.exeConf.blobstoreRateLimit > 0 && exeNode.exeConf.blobstoreBurstLimit > 0 {
		opts = append(opts, blob.WithRateLimit(float64(exeNode.exeConf.blobstoreRateLimit), exeNode.exeConf.blobstoreBurstLimit))
	}

	edsChannel := channels.ExecutionDataService
	if node.ObserverMode {
		edsChannel = channels.PublicExecutionDataService
	}
	bs, err := node.EngineRegistry.RegisterBlobService(edsChannel, exeNode.executionDataDatastore.Datastore(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to register blob service: %w", err)
	}
	exeNode.blobService = bs

	// add blobservice into ReadyDoneAware dependency passed to peer manager
	// this configures peer manager to wait for the blobservice to be ready before starting
	exeNode.blobserviceDependable.Init(bs)

	// blob service's lifecycle is managed by the network layer
	return &module.NoopReadyDoneAware{}, nil
}

func (exeNode *ExecutionNode) LoadBlockUploaderManager(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// blockDataUploader isn't a component, but needs to be initialized after the tracer, which is
	// a component.
	exeNode.blockDataUploader = uploader.NewManager(exeNode.builder.Tracer)
	return &module.NoopReadyDoneAware{}, nil
}

func (exeNode *ExecutionNode) LoadGCPBlockDataUploader(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// Since RetryableAsyncUploaderWrapper relies on executionDataService so we should create
	// it after execution data service is fully setup.
	if !exeNode.exeConf.enableBlockDataUpload || exeNode.exeConf.gcpBucketName == "" {
		// Since we don't have conditional component creation, we just use Noop one.
		// It's functions will be once per startup/shutdown - non-measurable performance penalty
		// blockDataUploader will stay nil and disable calling uploader at all
		return &module.NoopReadyDoneAware{}, nil
	}

	logger := node.Logger.With().Str("component_name", "gcp_block_data_uploader").Logger()
	gcpBucketUploader, err := uploader.NewGCPBucketUploader(
		context.Background(),
		exeNode.exeConf.gcpBucketName,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create GCP Bucket uploader: %w", err)
	}

	asyncUploader := uploader.NewAsyncUploader(
		gcpBucketUploader,
		blockdataUploaderRetryTimeout,
		blockDataUploaderMaxRetry,
		logger,
		exeNode.collector,
	)

	// Setting up RetryableUploader for GCP uploader
	// deprecated
	retryableUploader := uploader.NewBadgerRetryableUploaderWrapper(
		asyncUploader,
		node.Storage.Blocks,
		exeNode.commits,
		exeNode.collections,
		exeNode.events,
		exeNode.results,
		exeNode.txResults,
		store.NewComputationResultUploadStatus(node.ProtocolDB),
		execution_data.NewDownloader(exeNode.blobService),
		exeNode.collector)
	if retryableUploader == nil {
		return nil, errors.New("failed to create ComputationResult upload status store")
	}

	exeNode.blockDataUploader.AddUploader(retryableUploader)

	return retryableUploader, nil
}

func (exeNode *ExecutionNode) LoadS3BlockDataUploader(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	if !exeNode.exeConf.enableBlockDataUpload || exeNode.exeConf.s3BucketName == "" {
		// Since we don't have conditional component creation, we just use Noop one.
		// It's functions will be once per startup/shutdown - non-measurable performance penalty
		// blockDataUploader will stay nil and disable calling uploader at all
		return &module.NoopReadyDoneAware{}, nil
	}
	logger := node.Logger.With().Str("component_name", "s3_block_data_uploader").Logger()

	ctx := context.Background()
	config, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := s3.NewFromConfig(config)
	s3Uploader := uploader.NewS3Uploader(
		ctx,
		client,
		exeNode.exeConf.s3BucketName,
		logger,
	)
	asyncUploader := uploader.NewAsyncUploader(
		s3Uploader,
		blockdataUploaderRetryTimeout,
		blockDataUploaderMaxRetry,
		logger,
		exeNode.collector,
	)

	// We are not enabling RetryableUploader for S3 uploader for now. When we need upload
	// retry for multiple uploaders, we will need to use different BadgerDB key prefix.
	exeNode.blockDataUploader.AddUploader(asyncUploader)

	return asyncUploader, nil
}

func (exeNode *ExecutionNode) LoadProviderEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	if exeNode.blobService == nil {
		return nil, errors.New("blob service is not initialized")
	}

	var providerMetrics module.ExecutionDataProviderMetrics = metrics.NewNoopCollector()
	if node.MetricsEnabled {
		providerMetrics = metrics.NewExecutionDataProviderCollector()
	}

	executionDataProvider := exedataprovider.NewProvider(
		node.Logger,
		providerMetrics,
		execution_data.DefaultSerializer,
		exeNode.blobService,
		exeNode.executionDataTracker,
	)

	// in case node.FvmOptions already set a logger, we don't want to override it
	opts := append([]fvm.Option{
		fvm.WithLogger(
			node.Logger.With().Str("module", "FVM").Logger(),
		)},
		node.FvmOptions...,
	)

	opts = append(opts,
		computation.DefaultFVMOptions(
			node.RootChainID,
			exeNode.exeConf.computationConfig.ExtensiveTracing,
			exeNode.exeConf.scheduleCallbacksEnabled,
		)...,
	)

	vmCtx := fvm.NewContext(node.RootChainID.Chain(), opts...)

	var collector module.ExecutionMetrics
	collector = exeNode.collector
	if exeNode.exeConf.transactionExecutionMetricsEnabled {
		// inject the transaction execution metrics
		collector = exeNode.collector.WithTransactionCallback(
			func(dur time.Duration, stats module.TransactionExecutionResultStats, info module.TransactionExecutionResultInfo) {
				exeNode.metricsProvider.Collect(
					info.BlockID,
					info.BlockHeight,
					txmetrics.TransactionExecutionMetrics{
						TransactionID:          info.TransactionID,
						ExecutionTime:          dur,
						ExecutionEffortWeights: stats.ComputationIntensities,
					})
			})
	}

	ledgerViewCommitter := committer.NewLedgerViewCommitter(exeNode.ledgerStorage, node.Tracer)
	manager, err := computation.New(
		node.Logger,
		collector,
		node.Tracer,
		node.Me,
		computation.NewProtocolStateWrapper(node.State),
		vmCtx,
		ledgerViewCommitter,
		executionDataProvider,
		exeNode.exeConf.computationConfig,
	)
	if err != nil {
		return nil, err
	}
	exeNode.computationManager = manager

	if node.ObserverMode {
		exeNode.providerEngine = &exeprovider.NoopEngine{}
	} else {
		var chunkDataPackRequestQueueMetrics module.HeroCacheMetrics = metrics.NewNoopCollector()
		if node.HeroCacheMetricsEnable {
			chunkDataPackRequestQueueMetrics = metrics.ChunkDataPackRequestQueueMetricsFactory(node.MetricsRegisterer)
		}
		chdpReqQueue := queue.NewHeroStore(exeNode.exeConf.chunkDataPackRequestsCacheSize, node.Logger, chunkDataPackRequestQueueMetrics)
		exeNode.providerEngine, err = exeprovider.New(
			node.Logger,
			node.Tracer,
			node.EngineRegistry,
			node.State,
			exeNode.executionState,
			exeNode.collector,
			exeNode.checkAuthorizedAtBlock,
			chdpReqQueue,
			exeNode.exeConf.chunkDataPackRequestWorkers,
			exeNode.exeConf.chunkDataPackQueryTimeout,
			exeNode.exeConf.chunkDataPackDeliveryTimeout,
		)
		if err != nil {
			return nil, err
		}
	}

	// Get latest executed block and a view at that block
	ctx := context.Background()
	height, blockID, err := exeNode.executionState.GetLastExecutedBlockID(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get the latest executed block id at height %v: %w",
			height, err)
	}

	blockSnapshot, _, err := exeNode.executionState.CreateStorageSnapshot(blockID)
	if err != nil {
		return nil, fmt.Errorf("cannot create a storage snapshot at block %v at height %v : %w", blockID,
			height, err)
	}

	// Get the epoch counter from the smart contract at the last executed block.
	contractEpochCounter, err := getContractEpochCounter(
		exeNode.computationManager.VM(),
		vmCtx,
		blockSnapshot)
	// Failing to fetch the epoch counter from the smart contract is a fatal error.
	if err != nil {
		return nil, fmt.Errorf("cannot get epoch counter from the smart contract at block %s at height %v: %w",
			blockID.String(), height, err)
	}

	// Get the epoch counter from the protocol state, at the same block.
	// Failing to fetch the epoch, or counter for the epoch, from the protocol state is a fatal error.
	currentEpoch, err := node.State.AtBlockID(blockID).Epochs().Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch at block %s: %w", blockID.String(), err)
	}
	protocolStateEpochCounter := currentEpoch.Counter()

	l := node.Logger.With().
		Str("component", "provider engine").
		Uint64("contractEpochCounter", contractEpochCounter).
		Uint64("protocolStateEpochCounter", protocolStateEpochCounter).
		Str("blockID", blockID.String()).
		Uint64("height", height).
		Logger()

	if contractEpochCounter != protocolStateEpochCounter {
		// Do not error, because immediately following a spork they will be mismatching,
		// until the resetEpoch transaction is submitted.
		l.Warn().
			Msg("Epoch counter from the FlowEpoch smart contract and from the protocol state mismatch!")
	} else {
		l.Info().
			Msg("Epoch counter from the FlowEpoch smart contract and from the protocol state match.")
	}

	return exeNode.providerEngine, nil
}

func (exeNode *ExecutionNode) LoadAuthorizationCheckingFunction(
	node *NodeConfig,
) error {

	exeNode.checkAuthorizedAtBlock = func(blockID flow.Identifier) (bool, error) {
		return protocol.IsNodeAuthorizedAt(node.State.AtBlockID(blockID), node.Me.NodeID())
	}
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionDataDatastore(
	node *NodeConfig,
) (err error) {
	exeNode.executionDataDatastore, err = edstorage.CreateDatastoreManager(
		node.Logger, exeNode.exeConf.executionDataDir)
	if err != nil {
		return fmt.Errorf("could not create execution data datastore manager: %w", err)
	}

	exeNode.builder.ShutdownFunc(exeNode.executionDataDatastore.Close)
	return nil
}

func (exeNode *ExecutionNode) LoadBlobservicePeerManagerDependencies(node *NodeConfig) error {
	exeNode.blobserviceDependable = module.NewProxiedReadyDoneAware()
	exeNode.builder.PeerManagerDependencies.Add(exeNode.blobserviceDependable)
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionDataGetter(node *NodeConfig) error {
	exeNode.executionDataBlobstore = blobs.NewBlobstore(exeNode.executionDataDatastore.Datastore())
	exeNode.executionDataStore = execution_data.NewExecutionDataStore(exeNode.executionDataBlobstore, execution_data.DefaultSerializer)
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionState(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {

	chunkDataPackDB, err := storagepebble.SafeOpen(
		node.Logger.With().Str("pebbledb", "cdp").Logger(),
		exeNode.exeConf.chunkDataPackDir,
	)
	if err != nil {
		return nil, fmt.Errorf("could not open chunk data pack database: %w", err)
	}

	exeNode.builder.ShutdownFunc(func() error {
		if err := chunkDataPackDB.Close(); err != nil {
			return fmt.Errorf("error closing chunk data pack database: %w", err)
		}
		return nil
	})

	chunkDB := pebbleimpl.ToDB(chunkDataPackDB)
	storedChunkDataPacks := store.NewStoredChunkDataPacks(
		node.Metrics.Cache, chunkDB, exeNode.exeConf.chunkDataPackCacheSize)
	chunkDataPacks := store.NewChunkDataPacks(node.Metrics.Cache,
		node.ProtocolDB, storedChunkDataPacks, exeNode.collections, exeNode.exeConf.chunkDataPackCacheSize)

	getLatestFinalized := func() (uint64, error) {
		final, err := node.State.Final().Head()
		if err != nil {
			return 0, err
		}

		return final.Height, nil
	}
	exeNode.chunkDataPackDB = chunkDataPackDB
	exeNode.chunkDataPacks = chunkDataPacks

	// migrate execution data for last sealed and executed block

	exeNode.executionState = state.NewExecutionState(
		exeNode.ledgerStorage,
		exeNode.commits,
		node.Storage.Blocks,
		node.Storage.Headers,
		chunkDataPacks,
		exeNode.results,
		exeNode.myReceipts,
		exeNode.events,
		exeNode.serviceEvents,
		exeNode.txResults,
		node.ProtocolDB,
		getLatestFinalized,
		node.Tracer,
		exeNode.registerStore,
		exeNode.exeConf.enableStorehouse,
		node.StorageLockMgr,
	)

	height, _, err := exeNode.executionState.GetLastExecutedBlockID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("could not get last executed block: %w", err)
	}

	log.Info().Msgf("execution state last executed block height: %v", height)
	exeNode.collector.ExecutionLastExecutedBlockHeight(height)

	return &module.NoopReadyDoneAware{}, nil
}

func (exeNode *ExecutionNode) LoadStopControl(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	ver, err := build.Semver()
	if err != nil {
		err = fmt.Errorf("could not set semver version for stop control. "+
			"version %s is not semver compliant: %w", build.Version(), err)

		// The node would not know its own version. Without this the node would not know
		// how to reach to version boundaries.
		exeNode.builder.Logger.
			Err(err).
			Msg("error starting stop control")

		return nil, err
	}

	latestFinalizedBlock, err := node.State.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get latest finalized block: %w", err)
	}

	stopControl := stop.NewStopControl(
		exeNode.ingestionUnit,
		exeNode.exeConf.maxGracefulStopDuration,
		exeNode.builder.Logger,
		exeNode.executionState,
		node.Storage.Headers,
		node.Storage.VersionBeacons,
		ver,
		latestFinalizedBlock,
		// TODO: rename to exeNode.exeConf.executionStopped to make it more consistent
		exeNode.exeConf.pauseExecution,
		true,
	)
	// stopControl needs to consume BlockFinalized events.
	node.ProtocolEvents.AddConsumer(stopControl)

	exeNode.stopControl = stopControl

	return stopControl, nil
}

func (exeNode *ExecutionNode) LoadRegisterStore(
	node *NodeConfig,
) error {
	if !exeNode.exeConf.enableStorehouse {
		node.Logger.Info().Msg("register store disabled")
		exeNode.registerStore = nil
		return nil
	}

	registerStore, closer, err := storehouse.LoadRegisterStore(
		node.Logger,
		node.State,
		node.Storage.Headers,
		node.ProtocolEvents,
		node.LastFinalizedHeader.Height,
		exeNode.collector,
		exeNode.exeConf.registerDir,
		exeNode.exeConf.triedir,
		exeNode.exeConf.importCheckpointWorkerCount,
		bootstrap.ImportRegistersFromCheckpoint,
	)
	if err != nil {
		return err
	}

	if closer != nil {
		exeNode.builder.ShutdownFunc(closer.Close)
	}

	exeNode.registerStore = registerStore
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionStateLedger(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// Create ledger using factory
	ledgerStorage, err := ledgerfactory.NewLedger(ledgerfactory.Config{
		LedgerServiceAddr:     exeNode.exeConf.ledgerServiceAddr,
		LedgerMaxRequestSize:  exeNode.exeConf.ledgerMaxRequestSize,
		LedgerMaxResponseSize: exeNode.exeConf.ledgerMaxResponseSize,
		Triedir:               exeNode.exeConf.triedir,
		MTrieCacheSize:        exeNode.exeConf.mTrieCacheSize,
		CheckpointDistance:    exeNode.exeConf.checkpointDistance,
		CheckpointsToKeep:     exeNode.exeConf.checkpointsToKeep,
		MetricsRegisterer:     node.MetricsRegisterer,
		WALMetrics:            exeNode.collector,
		LedgerMetrics:         exeNode.collector,
		Logger:                node.Logger,
	}, exeNode.toTriggerCheckpoint)
	if err != nil {
		return nil, err
	}

	exeNode.ledgerStorage = ledgerStorage

	return exeNode.ledgerStorage, nil
}

func (exeNode *ExecutionNode) LoadExecutionDataPruner(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	sealed, err := node.State.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("cannot get the sealed block: %w", err)
	}

	trackerDir := filepath.Join(exeNode.exeConf.executionDataDir, "tracker")
	exeNode.executionDataTracker, err = tracker.OpenStorage(
		trackerDir,
		sealed.Height,
		node.Logger,
		tracker.WithPruneCallback(func(c cid.Cid) error {
			// TODO: use a proper context here
			return exeNode.executionDataBlobstore.DeleteBlob(context.TODO(), c)
		}),
	)
	if err != nil {
		return nil, err
	}

	// by default, pruning is disabled
	if exeNode.exeConf.executionDataPrunerHeightRangeTarget == 0 {
		return &module.NoopReadyDoneAware{}, nil
	}

	var prunerMetrics module.ExecutionDataPrunerMetrics = metrics.NewNoopCollector()
	if node.MetricsEnabled {
		prunerMetrics = metrics.NewExecutionDataPrunerCollector()
	}

	exeNode.executionDataPruner, err = pruner.NewPruner(
		node.Logger,
		prunerMetrics,
		exeNode.executionDataTracker,
		pruner.WithPruneCallback(func(ctx context.Context) error {
			return exeNode.executionDataDatastore.CollectGarbage(ctx)
		}),
		pruner.WithHeightRangeTarget(exeNode.exeConf.executionDataPrunerHeightRangeTarget),
		pruner.WithThreshold(exeNode.exeConf.executionDataPrunerThreshold),
	)
	return exeNode.executionDataPruner, err
}

func (exeNode *ExecutionNode) LoadExecutionDBPruner(node *NodeConfig) (module.ReadyDoneAware, error) {
	cfg := exepruner.PruningConfig{
		Threshold:                 exeNode.exeConf.pruningConfigThreshold,
		BatchSize:                 exeNode.exeConf.pruningConfigBatchSize,
		SleepAfterEachBatchCommit: exeNode.exeConf.pruningConfigSleepAfterCommit,
		SleepAfterEachIteration:   exeNode.exeConf.pruningConfigSleepAfterIteration,
	}

	return exepruner.NewChunkDataPackPruningEngine(
		node.Logger,
		exeNode.collector,
		node.State,
		node.ProtocolDB,
		node.Storage.Headers,
		exeNode.chunkDataPacks,
		exeNode.results,
		exeNode.chunkDataPackDB,
		cfg,
	), nil
}

func (exeNode *ExecutionNode) LoadCheckerEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	if !exeNode.exeConf.enableChecker {
		node.Logger.Warn().Msgf("checker engine is disabled")
		return &module.NoopReadyDoneAware{}, nil
	}

	node.Logger.Info().Msgf("checker engine is enabled")

	core := checker.NewCore(
		node.Logger,
		node.State,
		exeNode.executionState,
	)
	exeNode.checkerEng = checker.NewEngine(core)
	return exeNode.checkerEng, nil
}

func (exeNode *ExecutionNode) LoadIngestionEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	var colFetcher ingestion.CollectionFetcher
	var err error

	if node.ObserverMode {
		anID, err := flow.HexStringToIdentifier(exeNode.exeConf.publicAccessID)
		if err != nil {
			return nil, fmt.Errorf("could not parse public access ID: %w", err)
		}

		anNode, ok := exeNode.builder.IdentityProvider.ByNodeID(anID)
		if !ok {
			return nil, fmt.Errorf("could not find public access node with ID %s", anID)
		}

		if anNode.Role != flow.RoleAccess {
			return nil, fmt.Errorf("public access node with ID %s is not an access node", anID)
		}

		if anNode.IsEjected() {
			return nil, fmt.Errorf("public access node with ID %s is ejected", anID)
		}

		accessFetcher, err := fetcher.NewAccessCollectionFetcher(node.Logger, anNode.Address, anNode.NetworkPubKey, anNode.NodeID, node.RootChainID.Chain())
		if err != nil {
			return nil, fmt.Errorf("could not create access collection fetcher: %w", err)
		}
		colFetcher = accessFetcher
		exeNode.collectionRequester = accessFetcher
	} else {
		fifoStore, err := engine.NewFifoMessageStore(requester.DefaultEntityRequestCacheSize)
		if err != nil {
			return nil, fmt.Errorf("could not create requester store: %w", err)
		}
		reqEng, err := requester.New(node.Logger.With().Str("entity", "collection").Logger(), node.Metrics.Engine, node.EngineRegistry, node.Me, node.State,
			fifoStore,
			channels.RequestCollections,
			filter.Any,
			func() flow.Entity { return new(flow.Collection) },
			// we are manually triggering batches in execution, but lets still send off a batch once a minute, as a safety net for the sake of retries
			requester.WithBatchInterval(exeNode.exeConf.requestInterval),
			// we have observed execution nodes occasionally fail to retrieve collections using this engine, which can cause temporary execution halts
			// setting a retry maximum of 10s results in a much faster recovery from these faults (default is 2m)
			requester.WithRetryMaximum(10*time.Second),
		)

		if err != nil {
			return nil, fmt.Errorf("could not create requester engine: %w", err)
		}

		colFetcher = fetcher.NewCollectionFetcher(node.Logger, reqEng, node.State, exeNode.exeConf.onflowOnlyLNs)
		exeNode.collectionRequester = reqEng
	}

	var blockExecutedCallback ingestion.BlockExecutedCallback
	if exeNode.blockExecutedNotifier != nil {
		blockExecutedCallback = exeNode.blockExecutedNotifier.OnExecuted
	}

	_, core, err := ingestion.NewMachine(
		node.Logger,
		node.ProtocolEvents,
		exeNode.collectionRequester,
		colFetcher,
		node.Storage.Headers,
		node.Storage.Blocks,
		exeNode.collections,
		exeNode.executionState,
		node.State,
		exeNode.collector,
		exeNode.computationManager,
		exeNode.providerEngine,
		exeNode.blockDataUploader,
		exeNode.stopControl,
		blockExecutedCallback,
	)

	return core, err
}

// create scripts engine for handling script execution
func (exeNode *ExecutionNode) LoadScriptsEngine(node *NodeConfig) (module.ReadyDoneAware, error) {

	exeNode.scriptsEng = scripts.New(
		node.Logger,
		exeNode.computationManager.QueryExecutor(),
		exeNode.executionState,
	)

	return exeNode.scriptsEng, nil
}

func (exeNode *ExecutionNode) LoadTransactionExecutionMetrics(
	node *NodeConfig,
) (module.ReadyDoneAware, error) {
	lastFinalizedHeader := node.LastFinalizedHeader

	metricsProvider := txmetrics.NewTransactionExecutionMetricsProvider(
		node.Logger,
		exeNode.executionState,
		node.Storage.Headers,
		lastFinalizedHeader.Height,
		exeNode.exeConf.transactionExecutionMetricsBufferSize,
	)

	node.ProtocolEvents.AddConsumer(metricsProvider)
	exeNode.metricsProvider = metricsProvider
	return metricsProvider, nil
}

func (exeNode *ExecutionNode) LoadConsensusCommittee(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// initialize consensus committee's membership state
	// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
	// Note: node.Me.NodeID() is not part of the consensus exeNode.committee
	committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
	}
	node.ProtocolEvents.AddConsumer(committee)
	exeNode.committee = committee

	return committee, nil
}

func (exeNode *ExecutionNode) LoadFollowerCore(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// create a finalizer that handles updating the protocol
	// state when the follower detects newly finalized blocks
	final := finalizer.NewFinalizer(node.ProtocolDB.Reader(), node.Storage.Headers, exeNode.followerState, node.Tracer)

	finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
	if err != nil {
		return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
	}

	// creates a consensus follower with ingestEngine as the notifier
	// so that it gets notified upon each new finalized block
	exeNode.followerCore, err = consensus.NewFollower(
		node.Logger,
		node.Metrics.Mempool,
		node.Storage.Headers,
		final,
		exeNode.followerDistributor,
		node.FinalizedRootBlock.ToHeader(),
		node.RootQC,
		finalized,
		pending,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create follower core logic: %w", err)
	}

	return exeNode.followerCore, nil
}

func (exeNode *ExecutionNode) LoadFollowerEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	packer := signature.NewConsensusSigDataPacker(exeNode.committee)
	// initialize the verifier for the protocol consensus
	verifier := verification.NewCombinedVerifier(exeNode.committee, packer)
	validator := validator.New(exeNode.committee, verifier)

	var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
	if node.HeroCacheMetricsEnable {
		heroCacheCollector = metrics.FollowerCacheMetrics(node.MetricsRegisterer)
	}

	core, err := followereng.NewComplianceCore(
		node.Logger,
		node.Metrics.Mempool,
		heroCacheCollector,
		exeNode.followerDistributor,
		exeNode.followerState,
		exeNode.followerCore,
		validator,
		exeNode.syncCore,
		node.Tracer,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create follower core: %w", err)
	}

	exeNode.followerEng, err = followereng.NewComplianceLayer(
		node.Logger,
		node.EngineRegistry,
		node.Me,
		node.Metrics.Engine,
		node.Storage.Headers,
		node.LastFinalizedHeader,
		core,
		exeNode.followerDistributor,
		node.ComplianceConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create follower engine: %w", err)
	}

	return exeNode.followerEng, nil
}

func (exeNode *ExecutionNode) LoadCollectionRequesterEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
	// order for it to properly start and shut down, we should still return it as its own engine here, so it can
	// be handled by the scaffold.
	return exeNode.collectionRequester, nil
}

func (exeNode *ExecutionNode) LoadReceiptProviderEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	retrieve := func(blockID flow.Identifier) (flow.Entity, error) {
		return exeNode.myReceipts.MyReceipt(blockID)
	}

	var receiptRequestQueueMetric module.HeroCacheMetrics = metrics.NewNoopCollector()
	if node.HeroCacheMetricsEnable {
		receiptRequestQueueMetric = metrics.ReceiptRequestsQueueMetricFactory(node.MetricsRegisterer)
	}
	receiptRequestQueue := queue.NewHeroStore(exeNode.exeConf.receiptRequestsCacheSize, node.Logger, receiptRequestQueueMetric)

	engineRegister := node.EngineRegistry
	if node.ObserverMode {
		engineRegister = &underlay.NoopEngineRegister{}
	}
	eng, err := provider.New(
		node.Logger.With().Str("entity", "receipt").Logger(),
		node.Metrics.Engine,
		engineRegister,
		node.Me,
		node.State,
		receiptRequestQueue,
		exeNode.exeConf.receiptRequestWorkers,
		channels.ProvideReceiptsByBlockID,
		filter.And(
			filter.IsValidCurrentEpochParticipantOrJoining,
			filter.HasRole[flow.Identity](flow.RoleConsensus),
		),
		retrieve,
	)
	return eng, err
}

func (exeNode *ExecutionNode) LoadSynchronizationEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// initialize the synchronization engine
	spamConfig, err := synchronization.NewSpamDetectionConfig()
	if err != nil {
		return nil, fmt.Errorf("could not initialize spam detection config: %w", err)
	}

	exeNode.syncEngine, err = synchronization.New(
		node.Logger,
		node.Metrics.Engine,
		node.EngineRegistry,
		node.Me,
		node.State,
		node.Storage.Blocks,
		exeNode.followerEng,
		exeNode.syncCore,
		node.SyncEngineIdentifierProvider,
		spamConfig,
		exeNode.followerDistributor,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
	}

	return exeNode.syncEngine, nil
}

func (exeNode *ExecutionNode) LoadGrpcServer(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// maintain backwards compatibility with the deprecated flag
	if exeNode.exeConf.rpcConf.DeprecatedMaxMsgSize != 0 {
		node.Logger.Warn().Msg("A deprecated flag was specified (--rpc-max-message-size). Use --rpc-max-request-message-size and --rpc-max-response-message-size instead. This flag will be removed in a future release.")
		exeNode.exeConf.rpcConf.MaxRequestMsgSize = exeNode.exeConf.rpcConf.DeprecatedMaxMsgSize
		exeNode.exeConf.rpcConf.MaxResponseMsgSize = exeNode.exeConf.rpcConf.DeprecatedMaxMsgSize
	}
	return rpc.New(
		node.Logger,
		exeNode.exeConf.rpcConf,
		exeNode.scriptsEng,
		node.Storage.Headers,
		node.State,
		exeNode.eventsReader,
		exeNode.resultsReader,
		exeNode.txResultsReader,
		exeNode.commitsReader,
		exeNode.metricsProvider,
		node.RootChainID,
		signature.NewBlockSignerDecoder(exeNode.committee),
		exeNode.exeConf.apiRatelimits,
		exeNode.exeConf.apiBurstlimits,
	), nil
}

func (exeNode *ExecutionNode) LoadBackgroundIndexerEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	engine, created, err := storehouse.LoadBackgroundIndexerEngine(
		node.Logger,
		exeNode.exeConf.enableBackgroundStorehouseIndexing,
		node.State,
		node.Storage.Headers,
		node.ProtocolEvents,
		node.LastFinalizedHeader.Height,
		exeNode.collector,
		exeNode.exeConf.registerDir,
		exeNode.exeConf.triedir,
		exeNode.exeConf.importCheckpointWorkerCount,
		bootstrap.ImportRegistersFromCheckpoint,
		exeNode.executionDataStore,
		exeNode.resultsReader,
		exeNode.blockExecutedNotifier,
		exeNode.followerDistributor,
		exeNode.exeConf.backgroundIndexerHeightsPerSecond,
	)
	if err != nil {
		return nil, err
	}

	if !created {
		return &module.NoopReadyDoneAware{}, nil
	}

	exeNode.backgroundIndexerEngine = engine
	return engine, nil
}

func (exeNode *ExecutionNode) LoadBootstrapper(node *NodeConfig) error {

	// check if the execution database already exists
	bootstrapper := bootstrap.NewBootstrapper(node.Logger)

	// in order to support switching from badger to pebble in the middle of the spork,
	// we will check if the execution database has been bootstrapped by reading the state from badger db.
	// and if not, bootstrap both badger and pebble db.
	commit, bootstrapped, err := bootstrapper.IsBootstrapped(node.ProtocolDB)
	if err != nil {
		return fmt.Errorf("could not query database to know whether database has been bootstrapped: %w", err)
	}

	node.Logger.Info().Msgf("execution database bootstrapped: %v, commit: %v", bootstrapped, commit)

	// if the execution database does not exist, then we need to bootstrap the execution database.
	if !bootstrapped {

		err := wal.CheckpointHasRootHash(
			node.Logger,
			path.Join(node.BootstrapDir, modelbootstrap.DirnameExecutionState),
			modelbootstrap.FilenameWALRootCheckpoint,
			ledger.RootHash(node.RootSeal.FinalState),
		)
		if err != nil {
			return err
		}

		// when bootstrapping, the bootstrap folder must have a checkpoint file
		// we need to cover this file to the trie folder to restore the trie to restore the execution state.
		err = copyBootstrapState(node.BootstrapDir, exeNode.exeConf.triedir)
		if err != nil {
			return fmt.Errorf("could not load bootstrap state from checkpoint file: %w", err)
		}

		err = bootstrapper.BootstrapExecutionDatabase(node.StorageLockMgr, node.ProtocolDB, node.RootSeal)
		if err != nil {
			return fmt.Errorf("could not bootstrap execution database: %w", err)
		}
	} else {
		// if execution database has been bootstrapped, then the root statecommit must equal to the one
		// in the bootstrap folder
		if commit != node.RootSeal.FinalState {
			return fmt.Errorf("mismatching root statecommitment. database has state commitment: %x, "+
				"bootstap has statecommitment: %x",
				commit, node.RootSeal.FinalState)
		}
	}

	return nil
}

// getContractEpochCounter Gets the epoch counters from the FlowEpoch smart
// contract from the snapshot provided.
func getContractEpochCounter(
	vm fvm.VM,
	vmCtx fvm.Context,
	snapshot snapshot.StorageSnapshot,
) (
	uint64,
	error,
) {
	sc := systemcontracts.SystemContractsForChain(vmCtx.Chain.ChainID())

	// Generate the script to get the epoch counter from the FlowEpoch smart contract
	scriptCode := templates.GenerateGetCurrentEpochCounterScript(sc.AsTemplateEnv())
	script := fvm.Script(scriptCode)

	// execute the script
	_, output, err := vm.Run(vmCtx, script, snapshot)
	if err != nil {
		return 0, fmt.Errorf("could not read epoch counter, internal error while executing script: %w", err)
	}
	if output.Err != nil {
		return 0, fmt.Errorf("could not read epoch counter, script error: %w", output.Err)
	}
	if output.Value == nil {
		return 0, fmt.Errorf("could not read epoch counter, script returned no value")
	}

	epochCounter := output.Value.(cadence.UInt64)
	return uint64(epochCounter), nil
}

// copy the checkpoint files from the bootstrap folder to the execution state folder
// Checkpoint file is required to restore the trie, and has to be placed in the execution
// state folder.
// There are two ways to generate a checkpoint file:
//  1. From a clean state.
//     Refer to the code in the testcase: TestGenerateExecutionState
//  2. From a previous execution state
//     This is often used when sporking the network.
//     Use the execution-state-extract util commandline to generate a checkpoint file from
//     a previous checkpoint file
func copyBootstrapState(dir, trie string) error {
	filename := ""
	firstCheckpointFilename := "00000000"

	fileExists := func(fileName string) bool {
		_, err := os.Stat(filepath.Join(dir, modelbootstrap.DirnameExecutionState, fileName))
		return err == nil
	}

	// if there is a root checkpoint file, then copy that file over
	if fileExists(modelbootstrap.FilenameWALRootCheckpoint) {
		filename = modelbootstrap.FilenameWALRootCheckpoint
	} else if fileExists(firstCheckpointFilename) {
		// else if there is a checkpoint file, then copy that file over
		filename = firstCheckpointFilename
	} else {
		filePath := filepath.Join(dir, modelbootstrap.DirnameExecutionState, firstCheckpointFilename)

		// include absolute path of the missing file in the error message
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			absPath = filePath
		}

		return fmt.Errorf("execution state file not found: %v", absPath)
	}

	// copy from the bootstrap folder to the execution state folder
	from, to := path.Join(dir, modelbootstrap.DirnameExecutionState), trie

	log.Info().Str("dir", dir).Str("trie", trie).
		Msgf("linking checkpoint file %v from directory: %v, to: %v", filename, from, to)

	copiedFiles, err := wal.SoftlinkCheckpointFile(filename, from, to)
	if err != nil {
		return fmt.Errorf("can not link checkpoint file %s, from %s to %s, %w",
			filename, from, to, err)
	}

	for _, newPath := range copiedFiles {
		fmt.Printf("linked root checkpoint file from directory: %v, to: %v\n", from, newPath)
	}

	return nil
}

func logSysInfo(logger zerolog.Logger) error {

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("failed to get virtual memory: %w", err)
	}

	info, err := cpu.Info()
	if err != nil {
		return fmt.Errorf("failed to get cpu info: %w", err)
	}

	logicalCores, err := cpu.Counts(true)
	if err != nil {
		return fmt.Errorf("failed to get logical cores: %w", err)
	}

	physicalCores, err := cpu.Counts(false)
	if err != nil {
		return fmt.Errorf("failed to get physical cores: %w", err)
	}

	if len(info) == 0 {
		return fmt.Errorf("cpu info length is 0")
	}

	logger.Info().Msgf("CPU: ModelName=%s, MHz=%.0f, Family=%s, Model=%s, Stepping=%d, Microcode=%s, PhysicalCores=%d, LogicalCores=%d",
		info[0].ModelName, info[0].Mhz, info[0].Family, info[0].Model, info[0].Stepping, info[0].Microcode, physicalCores, logicalCores)

	logger.Info().Msgf("RAM: Total=%d, Free=%d", vmem.Total, vmem.Free)

	hostInfo, err := host.Info()
	if err != nil {
		return fmt.Errorf("failed to get platform info: %w", err)
	}
	logger.Info().Msgf("OS: OS=%s, Platform=%s, PlatformVersion=%s, KernelVersion=%s, Uptime: %d",
		hostInfo.OS, hostInfo.Platform, hostInfo.PlatformVersion, hostInfo.KernelVersion, hostInfo.Uptime)

	// goruntime.GOMAXPROCS(0) doesn't modify any settings.
	logger.Info().Msgf("GO: GoVersion=%s, GOMAXPROCS=%d, NumCPU=%d",
		goruntime.Version(), goruntime.GOMAXPROCS(0), goruntime.NumCPU())

	return nil
}
