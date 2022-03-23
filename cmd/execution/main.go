package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ipfs/go-bitswap"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	cpu "github.com/shirou/gopsutil/v3/cpu"
	mem "github.com/shirou/gopsutil/v3/mem"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/admin/commands"
	stateSyncCommands "github.com/onflow/flow-go/admin/commands/state_synchronization"
	uploaderCommands "github.com/onflow/flow-go/admin/commands/uploader"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
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
	"github.com/onflow/flow-go/engine/execution/computation/computer/uploader"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	exeprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/rpc"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/extralog"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	bootstrapFilenames "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
)

func main() {

	var (
		followerState                 protocol.MutableState
		ledgerStorage                 *ledger.Ledger
		events                        *storage.Events
		serviceEvents                 *storage.ServiceEvents
		txResults                     *storage.TransactionResults
		results                       *storage.ExecutionResults
		myReceipts                    *storage.MyExecutionReceipts
		providerEngine                *exeprovider.Engine
		checkerEng                    *checker.Engine
		syncCore                      *chainsync.Core
		pendingBlocks                 *buffer.PendingBlocks // used in follower engine
		deltas                        *ingestion.Deltas
		syncEngine                    *synchronization.Engine
		followerEng                   *followereng.Engine // to sync blocks from consensus nodes
		computationManager            *computation.Manager
		collectionRequester           *requester.Engine
		ingestionEng                  *ingestion.Engine
		finalizationDistributor       *pubsub.FinalizationDistributor
		finalizedHeader               *synchronization.FinalizedHeaderCache
		rpcConf                       rpc.Config
		err                           error
		executionState                state.ExecutionState
		triedir                       string
		executionDataDir              string
		collector                     module.ExecutionMetrics
		executionDataServiceCollector module.ExecutionDataServiceMetrics
		mTrieCacheSize                uint32
		transactionResultsCacheSize   uint
		checkpointDistance            uint
		checkpointsToKeep             uint
		stateDeltasLimit              uint
		cadenceExecutionCache         uint
		cadenceTracing                bool
		chdpCacheSize                 uint
		requestInterval               time.Duration
		preferredExeNodeIDStr         string
		syncByBlocks                  bool
		syncFast                      bool
		syncThreshold                 int
		extensiveLog                  bool
		pauseExecution                bool
		checkAuthorizedAtBlock        func(blockID flow.Identifier) (bool, error)
		diskWAL                       *wal.DiskWAL
		scriptLogThreshold            time.Duration
		chdpQueryTimeout              uint
		chdpDeliveryTimeout           uint
		enableBlockDataUpload         bool
		gcpBucketName                 string
		s3BucketName                  string
		blockDataUploaders            []uploader.Uploader
		blockDataUploaderMaxRetry     uint64 = 5
		blockdataUploaderRetryTimeout        = 1 * time.Second
		executionDataService          state_synchronization.ExecutionDataService
		executionDataCIDCache         state_synchronization.ExecutionDataCIDCache
		executionDataCIDCacheSize     uint = 100
		edsDatastoreTTL               time.Duration
	)

	nodeBuilder := cmd.FlowNode(flow.RoleExecution.String())
	nodeBuilder.
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.BoolVar(&rpcConf.RpcMetricsEnabled, "rpc-metrics-enabled", false, "whether to enable the rpc metrics")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			flags.StringVar(&executionDataDir, "execution-data-dir", filepath.Join(homedir, ".flow", "execution_data_blobstore"), "directory to use for Execution Data blobstore")
			flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 500, "cache size for MTrie")
			flags.UintVar(&checkpointDistance, "checkpoint-distance", 40, "number of WAL segments between checkpoints")
			flags.UintVar(&checkpointsToKeep, "checkpoints-to-keep", 5, "number of recent checkpoints to keep (0 to keep all)")
			flags.UintVar(&stateDeltasLimit, "state-deltas-limit", 100, "maximum number of state deltas in the memory pool")
			flags.UintVar(&cadenceExecutionCache, "cadence-execution-cache", computation.DefaultProgramsCacheSize, "cache size for Cadence execution")
			flags.BoolVar(&cadenceTracing, "cadence-tracing", false, "enables cadence runtime level tracing")
			flags.UintVar(&chdpCacheSize, "chdp-cache", storage.DefaultCacheSize, "cache size for Chunk Data Packs")
			flags.DurationVar(&requestInterval, "request-interval", 60*time.Second, "the interval between requests for the requester engine")
			flags.DurationVar(&scriptLogThreshold, "script-log-threshold", computation.DefaultScriptLogThreshold, "threshold for logging script execution")
			flags.StringVar(&preferredExeNodeIDStr, "preferred-exe-node-id", "", "node ID for preferred execution node used for state sync")
			flags.UintVar(&transactionResultsCacheSize, "transaction-results-cache-size", 10000, "number of transaction results to be cached")
			flags.BoolVar(&syncByBlocks, "sync-by-blocks", true, "deprecated, sync by blocks instead of execution state deltas")
			flags.BoolVar(&syncFast, "sync-fast", false, "fast sync allows execution node to skip fetching collection during state syncing, and rely on state syncing to catch up")
			flags.IntVar(&syncThreshold, "sync-threshold", 100, "the maximum number of sealed and unexecuted blocks before triggering state syncing")
			flags.BoolVar(&extensiveLog, "extensive-logging", false, "extensive logging logs tx contents and block headers")
			flags.UintVar(&chdpQueryTimeout, "chunk-data-pack-query-timeout-sec", 10, "number of seconds to determine a chunk data pack query being slow")
			flags.UintVar(&chdpDeliveryTimeout, "chunk-data-pack-delivery-timeout-sec", 10, "number of seconds to determine a chunk data pack response delivery being slow")
			flags.BoolVar(&pauseExecution, "pause-execution", false, "pause the execution. when set to true, no block will be executed, but still be able to serve queries")
			flags.BoolVar(&enableBlockDataUpload, "enable-blockdata-upload", false, "enable uploading block data to Cloud Bucket")
			flags.StringVar(&gcpBucketName, "gcp-bucket-name", "", "GCP Bucket name for block data uploader")
			flags.StringVar(&s3BucketName, "s3-bucket-name", "", "S3 Bucket name for block data uploader")
			flags.DurationVar(&edsDatastoreTTL, "execution-data-service-datastore-ttl", 0, "TTL for new blobs added to the execution data service blobstore")
		}).
		ValidateFlags(func() error {
			if enableBlockDataUpload {
				if gcpBucketName == "" && s3BucketName == "" {
					return fmt.Errorf("invalid flag. gcp-bucket-name or s3-bucket-name required when blockdata-uploader is enabled")
				}
			}
			return nil
		})

	if err = nodeBuilder.Initialize(); err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}

	nodeBuilder.
		AdminCommand("read-execution-data", func(config *cmd.NodeConfig) commands.AdminCommand {
			return stateSyncCommands.NewReadExecutionDataCommand(executionDataService)
		}).
		AdminCommand("set-uploader-enabled", func(config *cmd.NodeConfig) commands.AdminCommand {
			return uploaderCommands.NewToggleUploaderCommand()
		}).
		Module("mutable follower state", func(node *cmd.NodeConfig) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
			}
			followerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blocktimer.DefaultBlockTimer,
			)
			return err
		}).
		Module("hardware specs", func(node *cmd.NodeConfig) error {
			hwLogger := node.Logger.With().Str("system", "hardware").Logger()
			err = logHardware(hwLogger)
			if err != nil {
				hwLogger.Error().Err(err)
			}
			return nil
		}).
		Module("execution metrics", func(node *cmd.NodeConfig) error {
			collector = metrics.NewExecutionCollector(node.Tracer)
			return nil
		}).
		Module("execution data service metrics", func(node *cmd.NodeConfig) error {
			executionDataServiceCollector = metrics.NewExecutionDataServiceCollector()
			return nil
		}).
		Module("sync core", func(node *cmd.NodeConfig) error {
			syncCore, err = chainsync.New(node.Logger, chainsync.DefaultConfig())
			return err
		}).
		Module("execution receipts storage", func(node *cmd.NodeConfig) error {
			results = storage.NewExecutionResults(node.Metrics.Cache, node.DB)
			myReceipts = storage.NewMyExecutionReceipts(node.Metrics.Cache, node.DB, node.Storage.Receipts.(*storage.ExecutionReceipts))
			return nil
		}).
		Module("pending block cache", func(node *cmd.NodeConfig) error {
			pendingBlocks = buffer.NewPendingBlocks() // for following main chain consensus
			return nil
		}).
		Component("GCP block data uploader", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			if enableBlockDataUpload && gcpBucketName != "" {
				logger := node.Logger.With().Str("component_name", "gcp_block_data_uploader").Logger()
				gcpBucketUploader, err := uploader.NewGCPBucketUploader(
					context.Background(),
					gcpBucketName,
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
					collector,
				)

				blockDataUploaders = append(blockDataUploaders, asyncUploader)

				return asyncUploader, nil
			}

			// Since we don't have conditional component creation, we just use Noop one.
			// It's functions will be once per startup/shutdown - non-measurable performance penalty
			// blockDataUploader will stay nil and disable calling uploader at all
			return &module.NoopReadyDoneAware{}, nil
		}).
		Component("S3 block data uploader", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			if enableBlockDataUpload && s3BucketName != "" {
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
					s3BucketName,
					logger,
				)
				asyncUploader := uploader.NewAsyncUploader(
					s3Uploader,
					blockdataUploaderRetryTimeout,
					blockDataUploaderMaxRetry,
					logger,
					collector,
				)
				blockDataUploaders = append(blockDataUploaders, asyncUploader)

				return asyncUploader, nil
			}

			// Since we don't have conditional component creation, we just use Noop one.
			// It's functions will be once per startup/shutdown - non-measurable performance penalty
			// blockDataUploader will stay nil and disable calling uploader at all
			return &module.NoopReadyDoneAware{}, nil
		}).
		Module("state deltas mempool", func(node *cmd.NodeConfig) error {
			deltas, err = ingestion.NewDeltas(stateDeltasLimit)
			return err
		}).
		Module("authorization checking function", func(node *cmd.NodeConfig) error {
			checkAuthorizedAtBlock = func(blockID flow.Identifier) (bool, error) {
				return protocol.IsNodeAuthorizedAt(node.State.AtBlockID(blockID), node.Me.NodeID())
			}
			return nil
		}).
		Component("Write-Ahead Log", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			diskWAL, err = wal.NewDiskWAL(node.Logger.With().Str("subcomponent", "wal").Logger(), node.MetricsRegisterer, collector, triedir, int(mTrieCacheSize), pathfinder.PathByteSize, wal.SegmentSize)
			return diskWAL, err
		}).
		Component("execution state ledger", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// check if the execution database already exists
			bootstrapper := bootstrap.NewBootstrapper(node.Logger)

			commit, bootstrapped, err := bootstrapper.IsBootstrapped(node.DB)
			if err != nil {
				return nil, fmt.Errorf("could not query database to know whether database has been bootstrapped: %w", err)
			}

			// if the execution database does not exist, then we need to bootstrap the execution database.
			if !bootstrapped {
				// when bootstrapping, the bootstrap folder must have a checkpoint file
				// we need to cover this file to the trie folder to restore the trie to restore the execution state.
				err = copyBootstrapState(node.BootstrapDir, triedir)
				if err != nil {
					return nil, fmt.Errorf("could not load bootstrap state from checkpoint file: %w", err)
				}

				// TODO: check that the checkpoint file contains the root block's statecommit hash

				err = bootstrapper.BootstrapExecutionDatabase(node.DB, node.RootSeal.FinalState, node.RootBlock.Header)
				if err != nil {
					return nil, fmt.Errorf("could not bootstrap execution database: %w", err)
				}
			} else {
				// if execution database has been bootstrapped, then the root statecommit must equal to the one
				// in the bootstrap folder
				if commit != node.RootSeal.FinalState {
					return nil, fmt.Errorf("mismatching root statecommitment. database has state commitment: %x, "+
						"bootstap has statecommitment: %x",
						commit, node.RootSeal.FinalState)
				}
			}

			ledgerStorage, err = ledger.NewLedger(diskWAL, int(mTrieCacheSize), collector, node.Logger.With().Str("subcomponent", "ledger").Logger(), ledger.DefaultPathFinderVersion)
			return ledgerStorage, err
		}).
		Component("execution state ledger WAL compactor", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			checkpointer, err := ledgerStorage.Checkpointer()
			if err != nil {
				return nil, fmt.Errorf("cannot create checkpointer: %w", err)
			}
			compactor := wal.NewCompactor(checkpointer, 10*time.Second, checkpointDistance, checkpointsToKeep, node.Logger.With().Str("subcomponent", "checkpointer").Logger())

			return compactor, nil
		}).
		Component("execution data service", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			err := os.MkdirAll(executionDataDir, 0700)

			if err != nil {
				return nil, err
			}

			dsOpts := &badger.DefaultOptions
			dsOpts.TTL = edsDatastoreTTL

			ds, err := badger.NewDatastore(executionDataDir, dsOpts)

			if err != nil {
				return nil, err
			}

			nodeBuilder.ShutdownFunc(ds.Close)

			// TODO: if the node is not starting from the beginning of the spork, it may be useful to prepopulate
			// the cache with the existing Execution Data blob trees. Currently, the cache is empty every time the
			// node restarts, meaning that there will initially be no prioritization of requests.
			executionDataCIDCache = state_synchronization.NewExecutionDataCIDCache(executionDataCIDCacheSize)

			bs, err := node.Network.RegisterBlobService(
				engine.ExecutionDataService,
				ds,
				p2p.WithBitswapOptions(
					bitswap.WithTaskComparator(
						func(ta, tb *bitswap.TaskInfo) bool {
							ra, err := executionDataCIDCache.Get(ta.Cid)

							if err != nil {
								return false
							}

							rb, err := executionDataCIDCache.Get(tb.Cid)

							if err != nil {
								return true
							}

							if ra.BlobTreeRecord.BlockHeight > rb.BlobTreeRecord.BlockHeight {
								// more recent block has higher priority
								return true
							} else if ra.BlobTreeRecord.BlockID == rb.BlobTreeRecord.BlockID {
								// deeper node in the same blob tree has higher priority
								return ra.BlobTreeLocation.Height < rb.BlobTreeLocation.Height
							} else {
								return false
							}
						},
					),
				))

			if err != nil {
				return nil, err
			}

			eds := state_synchronization.NewExecutionDataService(
				&cbor.Codec{},
				compressor.NewLz4Compressor(),
				bs,
				executionDataServiceCollector,
				node.Logger,
			)

			executionDataService = eds

			return eds, nil
		}).
		Component("provider engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			extraLogPath := path.Join(triedir, "extralogs")
			err := os.MkdirAll(extraLogPath, 0777)
			if err != nil {
				return nil, fmt.Errorf("cannot create %s path for extra logs: %w", extraLogPath, err)
			}

			extralog.ExtraLogDumpPath = extraLogPath

			options := []runtime.Option{}
			if cadenceTracing {
				options = append(options, runtime.WithTracingEnabled(true))
			}
			rt := fvm.NewInterpreterRuntime(options...)

			vm := fvm.NewVirtualMachine(rt)
			vmCtx := fvm.NewContext(node.Logger, node.FvmOptions...)

			committer := committer.NewLedgerViewCommitter(ledgerStorage, node.Tracer)
			manager, err := computation.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Me,
				node.State,
				vm,
				vmCtx,
				cadenceExecutionCache,
				committer,
				scriptLogThreshold,
				blockDataUploaders,
				executionDataService,
				executionDataCIDCache,
			)
			if err != nil {
				return nil, err
			}
			computationManager = manager

			chunkDataPacks := storage.NewChunkDataPacks(node.Metrics.Cache, node.DB, node.Storage.Collections, chdpCacheSize)
			stateCommitments := storage.NewCommits(node.Metrics.Cache, node.DB)

			// Needed for gRPC server, make sure to assign to main scoped vars
			events = storage.NewEvents(node.Metrics.Cache, node.DB)
			serviceEvents = storage.NewServiceEvents(node.Metrics.Cache, node.DB)
			txResults = storage.NewTransactionResults(node.Metrics.Cache, node.DB, transactionResultsCacheSize)

			executionState = state.NewExecutionState(
				ledgerStorage,
				stateCommitments,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				chunkDataPacks,
				results,
				node.Storage.Receipts,
				myReceipts,
				events,
				serviceEvents,
				txResults,
				node.DB,
				node.Tracer,
			)

			providerEngine, err = exeprovider.New(
				node.Logger,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				executionState,
				collector,
				checkAuthorizedAtBlock,
				chdpQueryTimeout,
				chdpDeliveryTimeout,
			)
			if err != nil {
				return nil, err
			}

			// Get latest executed block and a view at that block
			ctx := context.Background()
			_, blockID, err := executionState.GetHighestExecutedBlockID(ctx)
			if err != nil {
				return nil, fmt.Errorf("cannot get the latest executed block id: %w", err)
			}
			stateCommit, err := executionState.StateCommitmentByBlockID(ctx, blockID)
			if err != nil {
				return nil, fmt.Errorf("cannot get the state comitment at latest executed block id %s: %w", blockID.String(), err)
			}
			blockView := executionState.NewView(stateCommit)

			// Get the epoch counter from the smart contract at the last executed block.
			contractEpochCounter, err := getContractEpochCounter(vm, vmCtx, blockView)
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

			return providerEngine, nil
		}).
		Component("checker engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			checkerEng = checker.New(
				node.Logger,
				node.State,
				executionState,
				node.Storage.Seals,
			)
			return checkerEng, nil
		}).
		Component("ingestion engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			collectionRequester, err = requester.New(node.Logger, node.Metrics.Engine, node.Network, node.Me, node.State,
				engine.RequestCollections,
				filter.Any,
				func() flow.Entity { return &flow.Collection{} },
				// we are manually triggering batches in execution, but lets still send off a batch once a minute, as a safety net for the sake of retries
				requester.WithBatchInterval(requestInterval),
				// consistency of collection can be checked by checking hash, and hash comes from trusted source (blocks from consensus follower)
				// hence we not need to check origin
				requester.WithValidateStaking(false),
			)

			preferredExeFilter := filter.Any
			preferredExeNodeID, err := flow.HexStringToIdentifier(preferredExeNodeIDStr)
			if err == nil {
				node.Logger.Info().Hex("prefered_exe_node_id", preferredExeNodeID[:]).Msg("starting with preferred exe sync node")
				preferredExeFilter = filter.HasNodeID(preferredExeNodeID)
			} else if err != nil && preferredExeNodeIDStr != "" {
				node.Logger.Debug().Str("prefered_exe_node_id_string", preferredExeNodeIDStr).Msg("could not parse exe node id, starting WITHOUT preferred exe sync node")
			}

			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				collectionRequester,
				node.State,
				node.Storage.Blocks,
				node.Storage.Collections,
				events,
				serviceEvents,
				txResults,
				computationManager,
				providerEngine,
				executionState,
				collector,
				node.Tracer,
				extensiveLog,
				preferredExeFilter,
				deltas,
				syncThreshold,
				syncFast,
				checkAuthorizedAtBlock,
				pauseExecution,
			)

			// TODO: we should solve these mutual dependencies better
			// => https://github.com/dapperlabs/flow-go/issues/4360
			collectionRequester = collectionRequester.WithHandle(ingestionEng.OnCollection)

			node.ProtocolEvents.AddConsumer(ingestionEng)

			return ingestionEng, err
		}).
		Component("follower engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, node.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)

			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState, node.Tracer)

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			packer := hotsignature.NewConsensusSigDataPacker(committee)
			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, packer)

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			finalizationDistributor = pubsub.NewFinalizationDistributor()
			finalizationDistributor.AddConsumer(checkerEng)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier, finalizationDistributor, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
			}

			followerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				followerState,
				pendingBlocks,
				followerCore,
				syncCore,
				node.Tracer,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("collection requester engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return collectionRequester, nil
		}).
		Component("receipt provider engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			retrieve := func(blockID flow.Identifier) (flow.Entity, error) { return myReceipts.MyReceipt(blockID) }
			eng, err := provider.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				engine.ProvideReceiptsByBlockID,
				filter.HasRole(flow.RoleConsensus),
				retrieve,
			)
			return eng, err
		}).
		Component("finalized snapshot", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			finalizedHeader, err = synchronization.NewFinalizedHeaderCache(node.Logger, node.State, finalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
			}

			return finalizedHeader, nil
		}).
		Component("synchronization engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// initialize the synchronization engine
			syncEngine, err = synchronization.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.Storage.Blocks,
				followerEng,
				syncCore,
				finalizedHeader,
				node.SyncEngineIdentifierProvider,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			return syncEngine, nil
		}).
		Component("grpc server", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng, node.Storage.Blocks, node.Storage.Headers, node.State, events, results, txResults, node.RootChainID)
			return rpcEng, nil
		})

	node, err := nodeBuilder.Build()
	if err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}

// getContractEpochCounter Gets the epoch counters from the FlowEpoch smart contract from the view provided.
func getContractEpochCounter(vm *fvm.VirtualMachine, vmCtx fvm.Context, view *delta.View) (uint64, error) {
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

	// Create empty programs cache
	p := programs.NewEmptyPrograms()

	// execute the script
	err = vm.Run(vmCtx, script, view, p)
	if err != nil {
		return 0, fmt.Errorf("could not read epoch counter, script internal error: %w", script.Err)
	}
	if script.Err != nil {
		return 0, fmt.Errorf("could not read epoch counter, script error: %w", script.Err)
	}
	if script.Value == nil {
		return 0, fmt.Errorf("could not read epoch counter, script returned no value")
	}

	epochCounter := script.Value.ToGoValue().(uint64)
	return epochCounter, nil
}

// copy the checkpoint files from the bootstrap folder to the execution state folder
// Checkpoint file is required to restore the trie, and has to be placed in the execution
// state folder.
// There are two ways to generate a checkpoint file:
// 1) From a clean state.
// 		Refer to the code in the testcase: TestGenerateExecutionState
// 2) From a previous execution state
// 		This is often used when sporking the network.
//    Use the execution-state-extract util commandline to generate a checkpoint file from
// 		a previous checkpoint file
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
	src := filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, filename)
	dst := filepath.Join(trie, filename)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// It's possible that the trie dir does not yet exist. If not this will create the the required path
	err = os.MkdirAll(trie, 0700)
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	fmt.Printf("copied bootstrap state file from: %v, to: %v\n", src, dst)

	return out.Close()
}

func logHardware(logger zerolog.Logger) error {

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

	logger.Info().Msgf("CPU: ModelName=%s, MHz=%.0f, Family=%s, Model=%s, Stepping=%d, PhysicalCores=%d, LogicalCores=%d",
		info[0].ModelName, info[0].Mhz, info[0].Family, info[0].Model, info[0].Stepping, physicalCores, logicalCores)

	logger.Info().Msgf("RAM: Total=%d, Free=%d", vmem.Total, vmem.Free)

	return nil
}
