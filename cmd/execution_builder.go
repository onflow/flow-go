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
	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/go-bitswap"
	"github.com/rs/zerolog"
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
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/execution/checker"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	exeprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/rpc"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	bootstrapFilenames "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	exedataprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/executiondatasync/pruner"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storageerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/procedure"
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

	collector              module.ExecutionMetrics
	executionState         state.ExecutionState
	followerState          protocol.FollowerState
	committee              hotstuff.DynamicCommittee
	ledgerStorage          *ledger.Ledger
	events                 *storage.Events
	serviceEvents          *storage.ServiceEvents
	txResults              *storage.TransactionResults
	results                *storage.ExecutionResults
	myReceipts             *storage.MyExecutionReceipts
	providerEngine         *exeprovider.Engine
	checkerEng             *checker.Engine
	syncCore               *chainsync.Core
	syncEngine             *synchronization.Engine
	followerCore           *hotstuff.FollowerLoop        // follower hotstuff logic
	followerEng            *followereng.ComplianceEngine // to sync blocks from consensus nodes
	computationManager     *computation.Manager
	collectionRequester    *requester.Engine
	ingestionEng           *ingestion.Engine
	followerDistributor    *pubsub.FollowerDistributor
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error)
	diskWAL                *wal.DiskWAL
	blockDataUploader      *uploader.Manager
	executionDataStore     execution_data.ExecutionDataStore
	toTriggerCheckpoint    *atomic.Bool      // create the checkpoint trigger to be controlled by admin tool, and listened by the compactor
	stopControl            *stop.StopControl // stop the node at given block height
	executionDataDatastore *badger.Datastore
	executionDataPruner    *pruner.Pruner
	executionDataBlobstore blobs.Blobstore
	executionDataTracker   tracker.Storage
	blobService            network.BlobService
	blobserviceDependable  *module.ProxiedReadyDoneAware
}

func (builder *ExecutionNodeBuilder) LoadComponentsAndModules() {

	exeNode := &ExecutionNode{
		builder:             builder.FlowNodeBuilder,
		exeConf:             builder.exeConf,
		toTriggerCheckpoint: atomic.NewBool(false),
	}

	builder.FlowNodeBuilder.
		AdminCommand("read-execution-data", func(config *NodeConfig) commands.AdminCommand {
			return stateSyncCommands.NewReadExecutionDataCommand(exeNode.executionDataStore)
		}).
		AdminCommand("trigger-checkpoint", func(config *NodeConfig) commands.AdminCommand {
			return executionCommands.NewTriggerCheckpointCommand(exeNode.toTriggerCheckpoint)
		}).
		AdminCommand("stop-at-height", func(config *NodeConfig) commands.AdminCommand {
			return executionCommands.NewStopAtHeightCommand(exeNode.stopControl)
		}).
		AdminCommand("set-uploader-enabled", func(config *NodeConfig) commands.AdminCommand {
			return uploaderCommands.NewToggleUploaderCommand(exeNode.blockDataUploader)
		}).
		AdminCommand("get-transactions", func(conf *NodeConfig) commands.AdminCommand {
			return storageCommands.NewGetTransactionsCommand(conf.State, conf.Storage.Payloads, conf.Storage.Collections)
		}).
		Module("mutable follower state", exeNode.LoadMutableFollowerState).
		Module("system specs", exeNode.LoadSystemSpecs).
		Module("execution metrics", exeNode.LoadExecutionMetrics).
		Module("sync core", exeNode.LoadSyncCore).
		Module("execution receipts storage", exeNode.LoadExecutionReceiptsStorage).
		Module("follower distributor", exeNode.LoadFollowerDistributor).
		Module("authorization checking function", exeNode.LoadAuthorizationCheckingFunction).
		Module("execution data datastore", exeNode.LoadExecutionDataDatastore).
		Module("execution data getter", exeNode.LoadExecutionDataGetter).
		Module("blobservice peer manager dependencies", exeNode.LoadBlobservicePeerManagerDependencies).
		Module("bootstrap", exeNode.LoadBootstrapper).
		Component("execution state ledger", exeNode.LoadExecutionStateLedger).

		// TODO: Modules should be able to depends on components
		// Because all modules are always bootstrapped first, before components,
		// its not possible to have a module depending on a Component.
		// This is the case for a StopControl which needs to query ExecutionState which needs execution state ledger.
		// I prefer to use dummy component now and keep the bootstrapping steps properly separated,
		// so it will be easier to follow and refactor later
		Component("execution state", exeNode.LoadExecutionState).
		Component("stop control", exeNode.LoadStopControl).
		Component("execution state ledger WAL compactor", exeNode.LoadExecutionStateLedgerWALCompactor).
		Component("execution data pruner", exeNode.LoadExecutionDataPruner).
		Component("blob service", exeNode.LoadBlobService).
		Component("block data upload manager", exeNode.LoadBlockUploaderManager).
		Component("GCP block data uploader", exeNode.LoadGCPBlockDataUploader).
		Component("S3 block data uploader", exeNode.LoadS3BlockDataUploader).
		Component("provider engine", exeNode.LoadProviderEngine).
		Component("checker engine", exeNode.LoadCheckerEngine).
		Component("ingestion engine", exeNode.LoadIngestionEngine).
		Component("consensus committee", exeNode.LoadConsensusCommittee).
		Component("follower core", exeNode.LoadFollowerCore).
		Component("follower engine", exeNode.LoadFollowerEngine).
		Component("collection requester engine", exeNode.LoadCollectionRequesterEngine).
		Component("receipt provider engine", exeNode.LoadReceiptProviderEngine).
		Component("synchronization engine", exeNode.LoadSynchronizationEngine).
		Component("grpc server", exeNode.LoadGrpcServer)
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
	var height uint64
	var blockID flow.Identifier
	err := node.DB.View(procedure.GetHighestExecutedBlock(&height, &blockID))
	if err != nil {
		// database has not been bootstrapped yet
		if errors.Is(err, storageerr.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("could not get highest executed block: %w", err)
	}

	exeNode.collector.ExecutionLastExecutedBlockHeight(height)
	return nil
}

func (exeNode *ExecutionNode) LoadSyncCore(node *NodeConfig) error {
	var err error
	exeNode.syncCore, err = chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector(node.RootChainID), node.RootChainID)
	return err
}

func (exeNode *ExecutionNode) LoadExecutionReceiptsStorage(
	node *NodeConfig,
) error {
	exeNode.results = storage.NewExecutionResults(node.Metrics.Cache, node.DB)
	exeNode.myReceipts = storage.NewMyExecutionReceipts(node.Metrics.Cache, node.DB, node.Storage.Receipts.(*storage.ExecutionReceipts))
	return nil
}

func (exeNode *ExecutionNode) LoadFollowerDistributor(node *NodeConfig) error {
	exeNode.followerDistributor = pubsub.NewFollowerDistributor()
	exeNode.followerDistributor.AddProposalViolationConsumer(notifications.NewSlashingViolationsConsumer(node.Logger))
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

			if id.Ejected {
				return nil, fmt.Errorf("allowed node ID %s is ejected", id.NodeID.String())
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

	if exeNode.exeConf.blobstoreRateLimit > 0 && exeNode.exeConf.blobstoreBurstLimit > 0 {
		opts = append(opts, blob.WithRateLimit(float64(exeNode.exeConf.blobstoreRateLimit), exeNode.exeConf.blobstoreBurstLimit))
	}

	bs, err := node.Network.RegisterBlobService(channels.ExecutionDataService, exeNode.executionDataDatastore, opts...)
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
	retryableUploader := uploader.NewBadgerRetryableUploaderWrapper(
		asyncUploader,
		node.Storage.Blocks,
		node.Storage.Commits,
		node.Storage.Collections,
		exeNode.events,
		exeNode.results,
		exeNode.txResults,
		storage.NewComputationResultUploadStatus(node.DB),
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
	vmCtx := fvm.NewContext(opts...)

	ledgerViewCommitter := committer.NewLedgerViewCommitter(exeNode.ledgerStorage, node.Tracer)
	manager, err := computation.New(
		node.Logger,
		exeNode.collector,
		node.Tracer,
		node.Me,
		node.State,
		vmCtx,
		ledgerViewCommitter,
		executionDataProvider,
		exeNode.exeConf.computationConfig,
	)
	if err != nil {
		return nil, err
	}
	exeNode.computationManager = manager

	var chunkDataPackRequestQueueMetrics module.HeroCacheMetrics = metrics.NewNoopCollector()
	if node.HeroCacheMetricsEnable {
		chunkDataPackRequestQueueMetrics = metrics.ChunkDataPackRequestQueueMetricsFactory(node.MetricsRegisterer)
	}
	chdpReqQueue := queue.NewHeroStore(exeNode.exeConf.chunkDataPackRequestsCacheSize, node.Logger, chunkDataPackRequestQueueMetrics)
	exeNode.providerEngine, err = exeprovider.New(
		node.Logger,
		node.Tracer,
		node.Network,
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

	// Get latest executed block and a view at that block
	ctx := context.Background()
	_, blockID, err := exeNode.executionState.GetHighestExecutedBlockID(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get the latest executed block id: %w",
			err)
	}
	stateCommit, err := exeNode.executionState.StateCommitmentByBlockID(
		ctx,
		blockID)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get the state commitment at latest executed block id %s: %w",
			blockID.String(),
			err)
	}
	blockSnapshot := exeNode.executionState.NewStorageSnapshot(stateCommit)

	// Get the epoch counter from the smart contract at the last executed block.
	contractEpochCounter, err := getContractEpochCounter(
		exeNode.computationManager.VM(),
		vmCtx,
		blockSnapshot)
	// Failing to fetch the epoch counter from the smart contract is a fatal error.
	if err != nil {
		return nil, fmt.Errorf("cannot get epoch counter from the smart contract at block %s: %w", blockID.String(), err)
	}

	// Get the epoch counter form the protocol state, at the same block.
	protocolStateEpochCounter, err := node.State.
		AtBlockID(blockID).
		Epochs().
		Current().
		Counter()
	// Failing to fetch the epoch counter from the protocol state is a fatal error.
	if err != nil {
		return nil, fmt.Errorf("cannot get epoch counter from the protocol state at block %s: %w", blockID.String(), err)
	}

	l := node.Logger.With().
		Str("component", "provider engine").
		Uint64("contractEpochCounter", contractEpochCounter).
		Uint64("protocolStateEpochCounter", protocolStateEpochCounter).
		Str("blockID", blockID.String()).
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
) error {
	datastoreDir := filepath.Join(exeNode.exeConf.executionDataDir, "blobstore")
	err := os.MkdirAll(datastoreDir, 0700)
	if err != nil {
		return err
	}
	dsOpts := &badger.DefaultOptions
	ds, err := badger.NewDatastore(datastoreDir, dsOpts)
	if err != nil {
		return err
	}
	exeNode.executionDataDatastore = ds
	exeNode.builder.ShutdownFunc(ds.Close)
	return nil
}

func (exeNode *ExecutionNode) LoadBlobservicePeerManagerDependencies(node *NodeConfig) error {
	exeNode.blobserviceDependable = module.NewProxiedReadyDoneAware()
	exeNode.builder.PeerManagerDependencies.Add(exeNode.blobserviceDependable)
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionDataGetter(node *NodeConfig) error {
	exeNode.executionDataBlobstore = blobs.NewBlobstore(exeNode.executionDataDatastore)
	exeNode.executionDataStore = execution_data.NewExecutionDataStore(exeNode.executionDataBlobstore, execution_data.DefaultSerializer)
	return nil
}

func (exeNode *ExecutionNode) LoadExecutionState(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {

	chunkDataPacks := storage.NewChunkDataPacks(node.Metrics.Cache, node.DB, node.Storage.Collections, exeNode.exeConf.chunkDataPackCacheSize)

	// Needed for gRPC server, make sure to assign to main scoped vars
	exeNode.events = storage.NewEvents(node.Metrics.Cache, node.DB)
	exeNode.serviceEvents = storage.NewServiceEvents(node.Metrics.Cache, node.DB)
	exeNode.txResults = storage.NewTransactionResults(node.Metrics.Cache, node.DB, exeNode.exeConf.transactionResultsCacheSize)

	exeNode.executionState = state.NewExecutionState(
		exeNode.ledgerStorage,
		node.Storage.Commits,
		node.Storage.Blocks,
		node.Storage.Headers,
		node.Storage.Collections,
		chunkDataPacks,
		exeNode.results,
		exeNode.myReceipts,
		exeNode.events,
		exeNode.serviceEvents,
		exeNode.txResults,
		node.DB,
		node.Tracer,
	)

	return &module.NoopReadyDoneAware{}, nil
}

func (exeNode *ExecutionNode) LoadStopControl(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	ver, err := build.SemverV2()
	if err != nil {
		ver = nil
		// TODO: In the future we want to error here, but for now we just log a warning.
		// This is because we currently have no strong guarantee that then node version
		// tag is semver compliant.
		exeNode.builder.Logger.Warn().
			Err(err).
			Msg("could not set semver version for stop control")
	}

	latestFinalizedBlock, err := node.State.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get latest finalized block: %w", err)
	}

	stopControl := stop.NewStopControl(
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

func (exeNode *ExecutionNode) LoadExecutionStateLedger(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	// DiskWal is a dependent component because we need to ensure
	// that all WAL updates are completed before closing opened WAL segment.
	var err error
	exeNode.diskWAL, err = wal.NewDiskWAL(node.Logger.With().Str("subcomponent", "wal").Logger(),
		node.MetricsRegisterer, exeNode.collector, exeNode.exeConf.triedir, int(exeNode.exeConf.mTrieCacheSize), pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize wal: %w", err)
	}

	exeNode.ledgerStorage, err = ledger.NewLedger(exeNode.diskWAL, int(exeNode.exeConf.mTrieCacheSize), exeNode.collector, node.Logger.With().Str("subcomponent",
		"ledger").Logger(), ledger.DefaultPathFinderVersion)
	return exeNode.ledgerStorage, err
}

func (exeNode *ExecutionNode) LoadExecutionStateLedgerWALCompactor(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	return ledger.NewCompactor(
		exeNode.ledgerStorage,
		exeNode.diskWAL,
		node.Logger.With().Str("subcomponent", "checkpointer").Logger(),
		uint(exeNode.exeConf.mTrieCacheSize),
		exeNode.exeConf.checkpointDistance,
		exeNode.exeConf.checkpointsToKeep,
		exeNode.toTriggerCheckpoint, // compactor will listen to the signal from admin tool for force triggering checkpointing
	)
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

func (exeNode *ExecutionNode) LoadCheckerEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	exeNode.checkerEng = checker.New(
		node.Logger,
		node.State,
		exeNode.executionState,
		node.Storage.Seals,
	)
	return exeNode.checkerEng, nil
}

func (exeNode *ExecutionNode) LoadIngestionEngine(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	var err error
	exeNode.collectionRequester, err = requester.New(node.Logger, node.Metrics.Engine, node.Network, node.Me, node.State,
		channels.RequestCollections,
		filter.Any,
		func() flow.Entity { return &flow.Collection{} },
		// we are manually triggering batches in execution, but lets still send off a batch once a minute, as a safety net for the sake of retries
		requester.WithBatchInterval(exeNode.exeConf.requestInterval),
		// consistency of collection can be checked by checking hash, and hash comes from trusted source (blocks from consensus follower)
		// hence we not need to check origin
		requester.WithValidateStaking(false),
	)

	if err != nil {
		return nil, fmt.Errorf("could not create requester engine: %w", err)
	}

	exeNode.ingestionEng, err = ingestion.New(
		node.Logger,
		node.Network,
		node.Me,
		exeNode.collectionRequester,
		node.State,
		node.Storage.Headers,
		node.Storage.Blocks,
		node.Storage.Collections,
		exeNode.events,
		exeNode.serviceEvents,
		exeNode.txResults,
		exeNode.computationManager,
		exeNode.providerEngine,
		exeNode.executionState,
		exeNode.collector,
		node.Tracer,
		exeNode.exeConf.extensiveLog,
		exeNode.checkAuthorizedAtBlock,
		exeNode.executionDataPruner,
		exeNode.blockDataUploader,
		exeNode.stopControl,
	)

	// TODO: we should solve these mutual dependencies better
	// => https://github.com/dapperlabs/flow-go/issues/4360
	exeNode.collectionRequester = exeNode.collectionRequester.WithHandle(exeNode.ingestionEng.OnCollection)

	node.ProtocolEvents.AddConsumer(exeNode.ingestionEng)

	return exeNode.ingestionEng, err
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
	final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, exeNode.followerState, node.Tracer)

	finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
	if err != nil {
		return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
	}

	exeNode.followerDistributor.AddFinalizationConsumer(exeNode.checkerEng)

	// creates a consensus follower with ingestEngine as the notifier
	// so that it gets notified upon each new finalized block
	exeNode.followerCore, err = consensus.NewFollower(
		node.Logger,
		node.Storage.Headers,
		final,
		exeNode.followerDistributor,
		node.FinalizedRootBlock.Header,
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
		node.Network,
		node.Me,
		node.Metrics.Engine,
		node.Storage.Headers,
		node.LastFinalizedHeader,
		core,
		node.ComplianceConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create follower engine: %w", err)
	}
	exeNode.followerDistributor.AddOnBlockFinalizedConsumer(exeNode.followerEng.OnFinalizedBlock)

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

	eng, err := provider.New(
		node.Logger,
		node.Metrics.Engine,
		node.Network,
		node.Me,
		node.State,
		receiptRequestQueue,
		exeNode.exeConf.receiptRequestWorkers,
		channels.ProvideReceiptsByBlockID,
		filter.And(
			filter.HasWeight(true),
			filter.HasRole(flow.RoleConsensus),
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
	var err error
	exeNode.syncEngine, err = synchronization.New(
		node.Logger,
		node.Metrics.Engine,
		node.Network,
		node.Me,
		node.State,
		node.Storage.Blocks,
		exeNode.followerEng,
		exeNode.syncCore,
		node.SyncEngineIdentifierProvider,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
	}
	exeNode.followerDistributor.AddFinalizationConsumer(exeNode.syncEngine)

	return exeNode.syncEngine, nil
}

func (exeNode *ExecutionNode) LoadGrpcServer(
	node *NodeConfig,
) (
	module.ReadyDoneAware,
	error,
) {
	return rpc.New(
		node.Logger,
		exeNode.exeConf.rpcConf,
		exeNode.ingestionEng,
		node.Storage.Headers,
		node.State,
		exeNode.events,
		exeNode.results,
		exeNode.txResults,
		node.Storage.Commits,
		node.RootChainID,
		signature.NewBlockSignerDecoder(exeNode.committee),
		exeNode.exeConf.apiRatelimits,
		exeNode.exeConf.apiBurstlimits,
	), nil
}

func (exeNode *ExecutionNode) LoadBootstrapper(node *NodeConfig) error {

	// check if the execution database already exists
	bootstrapper := bootstrap.NewBootstrapper(node.Logger)

	commit, bootstrapped, err := bootstrapper.IsBootstrapped(node.DB)
	if err != nil {
		return fmt.Errorf("could not query database to know whether database has been bootstrapped: %w", err)
	}

	// if the execution database does not exist, then we need to bootstrap the execution database.
	if !bootstrapped {
		// when bootstrapping, the bootstrap folder must have a checkpoint file
		// we need to cover this file to the trie folder to restore the trie to restore the execution state.
		err = copyBootstrapState(node.BootstrapDir, exeNode.exeConf.triedir)
		if err != nil {
			return fmt.Errorf("could not load bootstrap state from checkpoint file: %w", err)
		}

		// TODO: check that the checkpoint file contains the root block's statecommit hash

		err = bootstrapper.BootstrapExecutionDatabase(node.DB, node.RootSeal)
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
	// Get the address of the FlowEpoch smart contract
	sc, err := systemcontracts.SystemContractsForChain(vmCtx.Chain.ChainID())
	if err != nil {
		return 0, fmt.Errorf("could not get system contracts: %w", err)
	}
	address := sc.Epoch.Address

	// Generate the script to get the epoch counter from the FlowEpoch smart contract
	scriptCode := templates.GenerateGetCurrentEpochCounterScript(templates.Environment{
		EpochAddress: address.Hex(),
	})
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

	epochCounter := output.Value.ToGoValue().(uint64)
	return epochCounter, nil
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
		_, err := os.Stat(filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, fileName))
		return err == nil
	}

	// if there is a root checkpoint file, then copy that file over
	if fileExists(bootstrapFilenames.FilenameWALRootCheckpoint) {
		filename = bootstrapFilenames.FilenameWALRootCheckpoint
	} else if fileExists(firstCheckpointFilename) {
		// else if there is a checkpoint file, then copy that file over
		filename = firstCheckpointFilename
	} else {
		filePath := filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, firstCheckpointFilename)

		// include absolute path of the missing file in the error message
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			absPath = filePath
		}

		return fmt.Errorf("execution state file not found: %v", absPath)
	}

	// copy from the bootstrap folder to the execution state folder
	from, to := path.Join(dir, bootstrapFilenames.DirnameExecutionState), trie
	copiedFiles, err := wal.CopyCheckpointFile(filename, from, to)
	if err != nil {
		return fmt.Errorf("can not copy checkpoint file %s, from %s to %s",
			filename, from, to)
	}

	for _, newPath := range copiedFiles {
		fmt.Printf("copied root checkpoint file from directory: %v, to: %v\n", from, newPath)
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
