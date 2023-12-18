package node_builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	badger "github.com/ipfs/go-ds-badger2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/go-bitswap"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/admin/commands"
	stateSyncCommands "github.com/onflow/flow-go/admin/commands/state_synchronization"
	storageCommands "github.com/onflow/flow-go/admin/commands/storage"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	consensuspubsub "github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	rpcConnection "github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/access/state_stream"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	execdatacache "github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/unstaked"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	edrequester "github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/network"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/p2p/builder"
	p2pbuilderconfig "github.com/onflow/flow-go/network/p2p/builder/config"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	relaynet "github.com/onflow/flow-go/network/relay"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/network/underlay"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	pStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// AccessNodeBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Access node.
// The private network allows the staked nodes to communicate among themselves, while the public network allows the
// Observers and an Access node to communicate.
//
//                                 public network                           private network
//  +------------------------+
//  | Observer             1 |<--------------------------|
//  +------------------------+                           v
//  +------------------------+                         +--------------------+                 +------------------------+
//  | Observer             2 |<----------------------->| Staked Access Node |<--------------->| All other staked Nodes |
//  +------------------------+                         +--------------------+                 +------------------------+
//  +------------------------+                           ^
//  | Observer             3 |<--------------------------|
//  +------------------------+

// AccessNodeConfig defines all the user defined parameters required to bootstrap an access node
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type AccessNodeConfig struct {
	supportsObserver             bool // True if this is an Access node that supports observers and consensus follower engines
	collectionGRPCPort           uint
	executionGRPCPort            uint
	pingEnabled                  bool
	nodeInfoFile                 string
	apiRatelimits                map[string]int
	apiBurstlimits               map[string]int
	rpcConf                      rpc.Config
	stateStreamConf              statestreambackend.Config
	stateStreamFilterConf        map[string]int
	ExecutionNodeAddress         string // deprecated
	HistoricalAccessRPCs         []access.AccessAPIClient
	logTxTimeToFinalized         bool
	logTxTimeToExecuted          bool
	logTxTimeToFinalizedExecuted bool
	retryEnabled                 bool
	rpcMetricsEnabled            bool
	executionDataSyncEnabled     bool
	executionDataDir             string
	executionDataStartHeight     uint64
	executionDataConfig          edrequester.ExecutionDataConfig
	PublicNetworkConfig          PublicNetworkConfig
	TxResultCacheSize            uint
	TxErrorMessagesCacheSize     uint
	executionDataIndexingEnabled bool
	registersDBPath              string
	checkpointFile               string
	scriptExecutorConfig         query.QueryConfig
}

type PublicNetworkConfig struct {
	// NetworkKey crypto.PublicKey // TODO: do we need a different key for the public network?
	BindAddress string
	Network     network.EngineRegistry
	Metrics     module.NetworkMetrics
}

// DefaultAccessNodeConfig defines all the default values for the AccessNodeConfig
func DefaultAccessNodeConfig() *AccessNodeConfig {
	homedir, _ := os.UserHomeDir()
	return &AccessNodeConfig{
		supportsObserver:   false,
		collectionGRPCPort: 9000,
		executionGRPCPort:  9000,
		rpcConf: rpc.Config{
			UnsecureGRPCListenAddr: "0.0.0.0:9000",
			SecureGRPCListenAddr:   "0.0.0.0:9001",
			HTTPListenAddr:         "0.0.0.0:8000",
			CollectionAddr:         "",
			HistoricalAccessAddrs:  "",
			BackendConfig: backend.Config{
				CollectionClientTimeout:   3 * time.Second,
				ExecutionClientTimeout:    3 * time.Second,
				ConnectionPoolSize:        backend.DefaultConnectionPoolSize,
				MaxHeightRange:            backend.DefaultMaxHeightRange,
				PreferredExecutionNodeIDs: nil,
				FixedExecutionNodeIDs:     nil,
				CircuitBreakerConfig: rpcConnection.CircuitBreakerConfig{
					Enabled:        false,
					RestoreTimeout: 60 * time.Second,
					MaxFailures:    5,
					MaxRequests:    1,
				},
				ScriptExecutionMode: backend.ScriptExecutionModeExecutionNodesOnly.String(), // default to ENs only for now
			},
			RestConfig: rest.Config{
				ListenAddress: "",
				WriteTimeout:  rest.DefaultWriteTimeout,
				ReadTimeout:   rest.DefaultReadTimeout,
				IdleTimeout:   rest.DefaultIdleTimeout,
			},
			MaxMsgSize:     grpcutils.DefaultMaxMsgSize,
			CompressorName: grpcutils.NoCompressor,
		},
		stateStreamConf: statestreambackend.Config{
			MaxExecutionDataMsgSize: grpcutils.DefaultMaxMsgSize,
			ExecutionDataCacheSize:  state_stream.DefaultCacheSize,
			ClientSendTimeout:       state_stream.DefaultSendTimeout,
			ClientSendBufferSize:    state_stream.DefaultSendBufferSize,
			MaxGlobalStreams:        state_stream.DefaultMaxGlobalStreams,
			EventFilterConfig:       state_stream.DefaultEventFilterConfig,
			ResponseLimit:           state_stream.DefaultResponseLimit,
			HeartbeatInterval:       state_stream.DefaultHeartbeatInterval,
			RegisterIDsRequestLimit: state_stream.DefaultRegisterIDsRequestLimit,
		},
		stateStreamFilterConf:        nil,
		ExecutionNodeAddress:         "localhost:9000",
		logTxTimeToFinalized:         false,
		logTxTimeToExecuted:          false,
		logTxTimeToFinalizedExecuted: false,
		pingEnabled:                  false,
		retryEnabled:                 false,
		rpcMetricsEnabled:            false,
		nodeInfoFile:                 "",
		apiRatelimits:                nil,
		apiBurstlimits:               nil,
		TxResultCacheSize:            0,
		TxErrorMessagesCacheSize:     1000,
		PublicNetworkConfig: PublicNetworkConfig{
			BindAddress: cmd.NotSet,
			Metrics:     metrics.NewNoopCollector(),
		},
		executionDataSyncEnabled: true,
		executionDataDir:         filepath.Join(homedir, ".flow", "execution_data"),
		executionDataStartHeight: 0,
		executionDataConfig: edrequester.ExecutionDataConfig{
			InitialBlockHeight: 0,
			MaxSearchAhead:     edrequester.DefaultMaxSearchAhead,
			FetchTimeout:       edrequester.DefaultFetchTimeout,
			MaxFetchTimeout:    edrequester.DefaultMaxFetchTimeout,
			RetryDelay:         edrequester.DefaultRetryDelay,
			MaxRetryDelay:      edrequester.DefaultMaxRetryDelay,
		},
		executionDataIndexingEnabled: false,
		registersDBPath:              filepath.Join(homedir, ".flow", "execution_state"),
		checkpointFile:               cmd.NotSet,
		scriptExecutorConfig:         query.NewDefaultConfig(),
	}
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow access node
// It is composed of the FlowNodeBuilder, the AccessNodeConfig and contains all the components and modules needed for the
// access nodes
type FlowAccessNodeBuilder struct {
	*cmd.FlowNodeBuilder
	*AccessNodeConfig

	// components
	FollowerState              protocol.FollowerState
	SyncCore                   *chainsync.Core
	RpcEng                     *rpc.Engine
	FollowerDistributor        *consensuspubsub.FollowerDistributor
	CollectionRPC              access.AccessAPIClient
	TransactionTimings         *stdmap.TransactionTimings
	CollectionsToMarkFinalized *stdmap.Times
	CollectionsToMarkExecuted  *stdmap.Times
	BlocksToMarkExecuted       *stdmap.Times
	TransactionMetrics         *metrics.TransactionCollector
	RestMetrics                *metrics.RestCollector
	AccessMetrics              module.AccessMetrics
	PingMetrics                module.PingMetrics
	Committee                  hotstuff.DynamicCommittee
	Finalized                  *flow.Header // latest finalized block that the node knows of at startup time
	Pending                    []*flow.Header
	FollowerCore               module.HotStuffFollower
	Validator                  hotstuff.Validator
	ExecutionDataDownloader    execution_data.Downloader
	ExecutionDataRequester     state_synchronization.ExecutionDataRequester
	ExecutionDataStore         execution_data.ExecutionDataStore
	ExecutionDataCache         *execdatacache.ExecutionDataCache
	ExecutionIndexer           *indexer.Indexer
	ExecutionIndexerCore       *indexer.IndexerCore
	ScriptExecutor             *backend.ScriptExecutor
	RegistersAsyncStore        *execution.RegistersAsyncStore

	// The sync engine participants provider is the libp2p peer store for the access node
	// which is not available until after the network has started.
	// Hence, a factory function that needs to be called just before creating the sync engine
	SyncEngineParticipantsProviderFactory func() module.IdentifierProvider

	// engines
	IngestEng      *ingestion.Engine
	RequestEng     *requester.Engine
	FollowerEng    *followereng.ComplianceEngine
	SyncEng        *synceng.Engine
	StateStreamEng *statestreambackend.Engine

	// grpc servers
	secureGrpcServer      *grpcserver.GrpcServer
	unsecureGrpcServer    *grpcserver.GrpcServer
	stateStreamGrpcServer *grpcserver.GrpcServer

	stateStreamBackend *statestreambackend.StateStreamBackend
}

func (builder *FlowAccessNodeBuilder) buildFollowerState() *FlowAccessNodeBuilder {
	builder.Module("mutable follower state", func(node *cmd.NodeConfig) error {
		// For now, we only support state implementations from package badger.
		// If we ever support different implementations, the following can be replaced by a type-aware factory
		state, ok := node.State.(*badgerState.State)
		if !ok {
			return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
		}

		followerState, err := badgerState.NewFollowerState(
			node.Logger,
			node.Tracer,
			node.ProtocolEvents,
			state,
			node.Storage.Index,
			node.Storage.Payloads,
			blocktimer.DefaultBlockTimer,
		)
		builder.FollowerState = followerState

		return err
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildSyncCore() *FlowAccessNodeBuilder {
	builder.Module("sync core", func(node *cmd.NodeConfig) error {
		syncCore, err := chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector(node.RootChainID), node.RootChainID)
		builder.SyncCore = syncCore

		return err
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildCommittee() *FlowAccessNodeBuilder {
	builder.Component("committee", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// initialize consensus committee's membership state
		// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS committee
		// Note: node.Me.NodeID() is not part of the consensus committee
		committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
		node.ProtocolEvents.AddConsumer(committee)
		builder.Committee = committee

		return committee, err
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildLatestHeader() *FlowAccessNodeBuilder {
	builder.Module("latest header", func(node *cmd.NodeConfig) error {
		finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
		builder.Finalized, builder.Pending = finalized, pending

		return err
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildFollowerCore() *FlowAccessNodeBuilder {
	builder.Component("follower core", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// create a finalizer that will handle updating the protocol
		// state when the follower detects newly finalized blocks
		final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, builder.FollowerState, node.Tracer)

		packer := signature.NewConsensusSigDataPacker(builder.Committee)
		// initialize the verifier for the protocol consensus
		verifier := verification.NewCombinedVerifier(builder.Committee, packer)
		builder.Validator = hotstuffvalidator.New(builder.Committee, verifier)

		followerCore, err := consensus.NewFollower(
			node.Logger,
			node.Metrics.Mempool,
			node.Storage.Headers,
			final,
			builder.FollowerDistributor,
			node.FinalizedRootBlock.Header,
			node.RootQC,
			builder.Finalized,
			builder.Pending,
		)
		if err != nil {
			return nil, fmt.Errorf("could not initialize follower core: %w", err)
		}
		builder.FollowerCore = followerCore

		return builder.FollowerCore, nil
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildFollowerEngine() *FlowAccessNodeBuilder {
	builder.Component("follower engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
		if node.HeroCacheMetricsEnable {
			heroCacheCollector = metrics.FollowerCacheMetrics(node.MetricsRegisterer)
		}

		core, err := followereng.NewComplianceCore(
			node.Logger,
			node.Metrics.Mempool,
			heroCacheCollector,
			builder.FollowerDistributor,
			builder.FollowerState,
			builder.FollowerCore,
			builder.Validator,
			builder.SyncCore,
			node.Tracer,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower core: %w", err)
		}

		builder.FollowerEng, err = followereng.NewComplianceLayer(
			node.Logger,
			node.EngineRegistry,
			node.Me,
			node.Metrics.Engine,
			node.Storage.Headers,
			builder.Finalized,
			core,
			node.ComplianceConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}
		builder.FollowerDistributor.AddOnBlockFinalizedConsumer(builder.FollowerEng.OnFinalizedBlock)

		return builder.FollowerEng, nil
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildSyncEngine() *FlowAccessNodeBuilder {
	builder.Component("sync engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		spamConfig, err := synceng.NewSpamDetectionConfig()
		if err != nil {
			return nil, fmt.Errorf("could not initialize spam detection config: %w", err)
		}
		sync, err := synceng.New(
			node.Logger,
			node.Metrics.Engine,
			node.EngineRegistry,
			node.Me,
			node.State,
			node.Storage.Blocks,
			builder.FollowerEng,
			builder.SyncCore,
			builder.SyncEngineParticipantsProviderFactory(),
			spamConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create synchronization engine: %w", err)
		}
		builder.SyncEng = sync
		builder.FollowerDistributor.AddFinalizationConsumer(sync)

		return builder.SyncEng, nil
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) BuildConsensusFollower() *FlowAccessNodeBuilder {
	builder.
		buildFollowerState().
		buildSyncCore().
		buildCommittee().
		buildLatestHeader().
		buildFollowerCore().
		buildFollowerEngine().
		buildSyncEngine()

	return builder
}

func (builder *FlowAccessNodeBuilder) BuildExecutionSyncComponents() *FlowAccessNodeBuilder {
	var ds *badger.Datastore
	var bs network.BlobService
	var processedBlockHeight storage.ConsumerProgress
	var processedNotifications storage.ConsumerProgress
	var bsDependable *module.ProxiedReadyDoneAware
	var execDataDistributor *edrequester.ExecutionDataDistributor
	var execDataCacheBackend *herocache.BlockExecutionData
	var executionDataStoreCache *execdatacache.ExecutionDataCache

	// setup dependency chain to ensure indexer starts after the requester
	requesterDependable := module.NewProxiedReadyDoneAware()
	indexerDependencies := cmd.NewDependencyList()
	indexerDependencies.Add(requesterDependable)

	builder.
		AdminCommand("read-execution-data", func(config *cmd.NodeConfig) commands.AdminCommand {
			return stateSyncCommands.NewReadExecutionDataCommand(builder.ExecutionDataStore)
		}).
		Module("execution data datastore and blobstore", func(node *cmd.NodeConfig) error {
			datastoreDir := filepath.Join(builder.executionDataDir, "blobstore")
			err := os.MkdirAll(datastoreDir, 0700)
			if err != nil {
				return err
			}

			ds, err = badger.NewDatastore(datastoreDir, &badger.DefaultOptions)
			if err != nil {
				return err
			}

			builder.ShutdownFunc(func() error {
				if err := ds.Close(); err != nil {
					return fmt.Errorf("could not close execution data datastore: %w", err)
				}
				return nil
			})

			return nil
		}).
		Module("processed block height consumer progress", func(node *cmd.NodeConfig) error {
			// Note: progress is stored in the datastore's DB since that is where the jobqueue
			// writes execution data to.
			processedBlockHeight = bstorage.NewConsumerProgress(ds.DB, module.ConsumeProgressExecutionDataRequesterBlockHeight)
			return nil
		}).
		Module("processed notifications consumer progress", func(node *cmd.NodeConfig) error {
			// Note: progress is stored in the datastore's DB since that is where the jobqueue
			// writes execution data to.
			processedNotifications = bstorage.NewConsumerProgress(ds.DB, module.ConsumeProgressExecutionDataRequesterNotification)
			return nil
		}).
		Module("blobservice peer manager dependencies", func(node *cmd.NodeConfig) error {
			bsDependable = module.NewProxiedReadyDoneAware()
			builder.PeerManagerDependencies.Add(bsDependable)
			return nil
		}).
		Module("execution datastore", func(node *cmd.NodeConfig) error {
			blobstore := blobs.NewBlobstore(ds)
			builder.ExecutionDataStore = execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)
			return nil
		}).
		Module("execution data cache", func(node *cmd.NodeConfig) error {
			var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
			if builder.HeroCacheMetricsEnable {
				heroCacheCollector = metrics.AccessNodeExecutionDataCacheMetrics(builder.MetricsRegisterer)
			}

			execDataCacheBackend = herocache.NewBlockExecutionData(builder.stateStreamConf.ExecutionDataCacheSize, builder.Logger, heroCacheCollector)

			// Execution Data cache that uses a blobstore as the backend (instead of a downloader)
			// This ensures that it simply returns a not found error if the blob doesn't exist
			// instead of attempting to download it from the network.
			executionDataStoreCache = execdatacache.NewExecutionDataCache(
				builder.ExecutionDataStore,
				builder.Storage.Headers,
				builder.Storage.Seals,
				builder.Storage.Results,
				execDataCacheBackend,
			)

			return nil
		}).
		Component("execution data service", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			opts := []network.BlobServiceOption{
				blob.WithBitswapOptions(
					// Only allow block requests from staked ENs and ANs
					bitswap.WithPeerBlockRequestFilter(
						blob.AuthorizedRequester(nil, builder.IdentityProvider, builder.Logger),
					),
					bitswap.WithTracer(
						blob.NewTracer(node.Logger.With().Str("blob_service", channels.ExecutionDataService.String()).Logger()),
					),
				),
			}

			var err error
			bs, err = node.EngineRegistry.RegisterBlobService(channels.ExecutionDataService, ds, opts...)
			if err != nil {
				return nil, fmt.Errorf("could not register blob service: %w", err)
			}

			// add blobservice into ReadyDoneAware dependency passed to peer manager
			// this starts the blob service and configures peer manager to wait for the blobservice
			// to be ready before starting
			bsDependable.Init(bs)

			builder.ExecutionDataDownloader = execution_data.NewDownloader(bs)

			return builder.ExecutionDataDownloader, nil
		}).
		Component("execution data requester", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// Validation of the start block height needs to be done after loading state
			if builder.executionDataStartHeight > 0 {
				if builder.executionDataStartHeight <= builder.FinalizedRootBlock.Header.Height {
					return nil, fmt.Errorf(
						"execution data start block height (%d) must be greater than the root block height (%d)",
						builder.executionDataStartHeight, builder.FinalizedRootBlock.Header.Height)
				}

				latestSeal, err := builder.State.Sealed().Head()
				if err != nil {
					return nil, fmt.Errorf("failed to get latest sealed height")
				}

				// Note: since the root block of a spork is also sealed in the root protocol state, the
				// latest sealed height is always equal to the root block height. That means that at the
				// very beginning of a spork, this check will always fail. Operators should not specify
				// an InitialBlockHeight when starting from the beginning of a spork.
				if builder.executionDataStartHeight > latestSeal.Height {
					return nil, fmt.Errorf(
						"execution data start block height (%d) must be less than or equal to the latest sealed block height (%d)",
						builder.executionDataStartHeight, latestSeal.Height)
				}

				// executionDataStartHeight is provided as the first block to sync, but the
				// requester expects the initial last processed height, which is the first height - 1
				builder.executionDataConfig.InitialBlockHeight = builder.executionDataStartHeight - 1
			} else {
				builder.executionDataConfig.InitialBlockHeight = builder.FinalizedRootBlock.Header.Height
			}

			execDataDistributor = edrequester.NewExecutionDataDistributor()

			// Execution Data cache with a downloader as the backend. This is used by the requester
			// to download and cache execution data for each block. It shares a cache backend instance
			// with the datastore implementation.
			executionDataCache := execdatacache.NewExecutionDataCache(
				builder.ExecutionDataDownloader,
				builder.Storage.Headers,
				builder.Storage.Seals,
				builder.Storage.Results,
				execDataCacheBackend,
			)

			r, err := edrequester.New(
				builder.Logger,
				metrics.NewExecutionDataRequesterCollector(),
				builder.ExecutionDataDownloader,
				executionDataCache,
				processedBlockHeight,
				processedNotifications,
				builder.State,
				builder.Storage.Headers,
				builder.executionDataConfig,
				execDataDistributor,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create execution data requester: %w", err)
			}
			builder.ExecutionDataRequester = r

			builder.FollowerDistributor.AddOnBlockFinalizedConsumer(builder.ExecutionDataRequester.OnBlockFinalized)

			// add requester into ReadyDoneAware dependency passed to indexer. This allows the indexer
			// to wait for the requester to be ready before starting.
			requesterDependable.Init(builder.ExecutionDataRequester)

			return builder.ExecutionDataRequester, nil
		})

	if builder.executionDataIndexingEnabled {
		var indexedBlockHeight storage.ConsumerProgress

		builder.
			AdminCommand("execute-script", func(config *cmd.NodeConfig) commands.AdminCommand {
				return stateSyncCommands.NewExecuteScriptCommand(builder.ScriptExecutor)
			}).
			Module("indexed block height consumer progress", func(node *cmd.NodeConfig) error {
				// Note: progress is stored in the MAIN db since that is where indexed execution data is stored.
				indexedBlockHeight = bstorage.NewConsumerProgress(builder.DB, module.ConsumeProgressExecutionDataIndexerBlockHeight)
				return nil
			}).
			Module("events storage", func(node *cmd.NodeConfig) error {
				builder.Storage.Events = bstorage.NewEvents(node.Metrics.Cache, node.DB)
				return nil
			}).
			Module("transaction results storage", func(node *cmd.NodeConfig) error {
				builder.Storage.LightTransactionResults = bstorage.NewLightTransactionResults(node.Metrics.Cache, node.DB, bstorage.DefaultCacheSize)
				return nil
			}).
			DependableComponent("execution data indexer", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
				// Note: using a DependableComponent here to ensure that the indexer does not block
				// other components from starting while bootstrapping the register db since it may
				// take hours to complete.

				pdb, err := pStorage.OpenRegisterPebbleDB(builder.registersDBPath)
				if err != nil {
					return nil, fmt.Errorf("could not open registers db: %w", err)
				}
				builder.ShutdownFunc(func() error {
					return pdb.Close()
				})

				bootstrapped, err := pStorage.IsBootstrapped(pdb)
				if err != nil {
					return nil, fmt.Errorf("could not check if registers db is bootstrapped: %w", err)
				}

				if !bootstrapped {
					checkpointFile := builder.checkpointFile
					if checkpointFile == cmd.NotSet {
						checkpointFile = path.Join(builder.BootstrapDir, bootstrap.PathRootCheckpoint)
					}

					// currently, the checkpoint must be from the root block.
					// read the root hash from the provided checkpoint and verify it matches the
					// state commitment from the root snapshot.
					err := wal.CheckpointHasRootHash(
						node.Logger,
						"", // checkpoint file already full path
						checkpointFile,
						ledger.RootHash(node.RootSeal.FinalState),
					)
					if err != nil {
						return nil, fmt.Errorf("could not verify checkpoint file: %w", err)
					}

					checkpointHeight := builder.SealedRootBlock.Header.Height

					buutstrap, err := pStorage.NewRegisterBootstrap(pdb, checkpointFile, checkpointHeight, builder.Logger)
					if err != nil {
						return nil, fmt.Errorf("could not create registers bootstrapper: %w", err)
					}

					// TODO: find a way to hook a context up to this to allow a graceful shutdown
					workerCount := 10
					err = buutstrap.IndexCheckpointFile(context.Background(), workerCount)
					if err != nil {
						return nil, fmt.Errorf("could not load checkpoint file: %w", err)
					}
				}

				registers, err := pStorage.NewRegisters(pdb)
				if err != nil {
					return nil, fmt.Errorf("could not create registers storage: %w", err)
				}

				builder.Storage.RegisterIndex = registers

				indexerCore, err := indexer.New(
					builder.Logger,
					metrics.NewExecutionStateIndexerCollector(),
					builder.DB,
					builder.Storage.RegisterIndex,
					builder.Storage.Headers,
					builder.Storage.Events,
					builder.Storage.LightTransactionResults,
				)
				if err != nil {
					return nil, err
				}
				builder.ExecutionIndexerCore = indexerCore

				// execution state worker uses a jobqueue to process new execution data and indexes it by using the indexer.
				builder.ExecutionIndexer, err = indexer.NewIndexer(
					builder.Logger,
					registers.FirstHeight(),
					registers,
					indexerCore,
					executionDataStoreCache,
					builder.ExecutionDataRequester.HighestConsecutiveHeight,
					indexedBlockHeight,
				)
				if err != nil {
					return nil, err
				}

				err = builder.RegistersAsyncStore.InitDataAvailable(registers)
				if err != nil {
					return nil, err
				}

				// setup requester to notify indexer when new execution data is received
				execDataDistributor.AddOnExecutionDataReceivedConsumer(builder.ExecutionIndexer.OnExecutionData)

				// create script execution module, this depends on the indexer being initialized and the
				// having the register storage bootstrapped
				scripts, err := execution.NewScripts(
					builder.Logger,
					metrics.NewExecutionCollector(builder.Tracer),
					builder.RootChainID,
					query.NewProtocolStateWrapper(builder.State),
					builder.Storage.Headers,
					builder.ExecutionIndexerCore.RegisterValue,
					builder.scriptExecutorConfig,
				)
				if err != nil {
					return nil, err
				}

				builder.ScriptExecutor.InitReporter(builder.ExecutionIndexer, scripts)

				return builder.ExecutionIndexer, nil
			}, indexerDependencies)
	}

	if builder.stateStreamConf.ListenAddr != "" {
		builder.Component("exec state stream engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			for key, value := range builder.stateStreamFilterConf {
				switch key {
				case "EventTypes":
					builder.stateStreamConf.MaxEventTypes = value
				case "Addresses":
					builder.stateStreamConf.MaxAddresses = value
				case "Contracts":
					builder.stateStreamConf.MaxContracts = value
				}
			}
			builder.stateStreamConf.RpcMetricsEnabled = builder.rpcMetricsEnabled

			highestAvailableHeight, err := builder.ExecutionDataRequester.HighestConsecutiveHeight()
			if err != nil {
				return nil, fmt.Errorf("could not get highest consecutive height: %w", err)
			}
			broadcaster := engine.NewBroadcaster()

			builder.stateStreamBackend, err = statestreambackend.New(
				node.Logger,
				builder.stateStreamConf,
				node.State,
				node.Storage.Headers,
				node.Storage.Seals,
				node.Storage.Results,
				builder.ExecutionDataStore,
				executionDataStoreCache,
				broadcaster,
				builder.executionDataConfig.InitialBlockHeight,
				highestAvailableHeight,
				builder.RegistersAsyncStore)
			if err != nil {
				return nil, fmt.Errorf("could not create state stream backend: %w", err)
			}

			stateStreamEng, err := statestreambackend.NewEng(
				node.Logger,
				builder.stateStreamConf,
				executionDataStoreCache,
				node.Storage.Headers,
				node.RootChainID,
				builder.stateStreamGrpcServer,
				builder.stateStreamBackend,
				broadcaster,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create state stream engine: %w", err)
			}
			builder.StateStreamEng = stateStreamEng

			execDataDistributor.AddOnExecutionDataReceivedConsumer(builder.StateStreamEng.OnExecutionData)

			return builder.StateStreamEng, nil
		})
	}

	return builder
}

func FlowAccessNode(nodeBuilder *cmd.FlowNodeBuilder) *FlowAccessNodeBuilder {
	dist := consensuspubsub.NewFollowerDistributor()
	dist.AddProposalViolationConsumer(notifications.NewSlashingViolationsConsumer(nodeBuilder.Logger))
	return &FlowAccessNodeBuilder{
		AccessNodeConfig:    DefaultAccessNodeConfig(),
		FlowNodeBuilder:     nodeBuilder,
		FollowerDistributor: dist,
	}
}

func (builder *FlowAccessNodeBuilder) ParseFlags() error {

	builder.BaseFlags()

	builder.extraFlags()

	return builder.ParseAndPrintFlags()
}

func (builder *FlowAccessNodeBuilder) extraFlags() {
	builder.ExtraFlags(func(flags *pflag.FlagSet) {
		defaultConfig := DefaultAccessNodeConfig()

		flags.UintVar(&builder.collectionGRPCPort, "collection-ingress-port", defaultConfig.collectionGRPCPort, "the grpc ingress port for all collection nodes")
		flags.UintVar(&builder.executionGRPCPort, "execution-ingress-port", defaultConfig.executionGRPCPort, "the grpc ingress port for all execution nodes")
		flags.StringVarP(&builder.rpcConf.UnsecureGRPCListenAddr,
			"rpc-addr",
			"r",
			defaultConfig.rpcConf.UnsecureGRPCListenAddr,
			"the address the unsecured gRPC server listens on")
		flags.StringVar(&builder.rpcConf.SecureGRPCListenAddr,
			"secure-rpc-addr",
			defaultConfig.rpcConf.SecureGRPCListenAddr,
			"the address the secure gRPC server listens on")
		flags.StringVar(&builder.stateStreamConf.ListenAddr,
			"state-stream-addr",
			defaultConfig.stateStreamConf.ListenAddr,
			"the address the state stream server listens on (if empty the server will not be started)")
		flags.StringVarP(&builder.rpcConf.HTTPListenAddr, "http-addr", "h", defaultConfig.rpcConf.HTTPListenAddr, "the address the http proxy server listens on")
		flags.StringVar(&builder.rpcConf.RestConfig.ListenAddress,
			"rest-addr",
			defaultConfig.rpcConf.RestConfig.ListenAddress,
			"the address the REST server listens on (if empty the REST server will not be started)")
		flags.DurationVar(&builder.rpcConf.RestConfig.WriteTimeout,
			"rest-write-timeout",
			defaultConfig.rpcConf.RestConfig.WriteTimeout,
			"timeout to use when writing REST response")
		flags.DurationVar(&builder.rpcConf.RestConfig.ReadTimeout,
			"rest-read-timeout",
			defaultConfig.rpcConf.RestConfig.ReadTimeout,
			"timeout to use when reading REST request headers")
		flags.DurationVar(&builder.rpcConf.RestConfig.IdleTimeout, "rest-idle-timeout", defaultConfig.rpcConf.RestConfig.IdleTimeout, "idle timeout for REST connections")
		flags.StringVarP(&builder.rpcConf.CollectionAddr,
			"static-collection-ingress-addr",
			"",
			defaultConfig.rpcConf.CollectionAddr,
			"the address (of the collection node) to send transactions to")
		flags.StringVarP(&builder.ExecutionNodeAddress,
			"script-addr",
			"s",
			defaultConfig.ExecutionNodeAddress,
			"the address (of the execution node) forward the script to")
		flags.StringVarP(&builder.rpcConf.HistoricalAccessAddrs,
			"historical-access-addr",
			"",
			defaultConfig.rpcConf.HistoricalAccessAddrs,
			"comma separated rpc addresses for historical access nodes")
		flags.DurationVar(&builder.rpcConf.BackendConfig.CollectionClientTimeout,
			"collection-client-timeout",
			defaultConfig.rpcConf.BackendConfig.CollectionClientTimeout,
			"grpc client timeout for a collection node")
		flags.DurationVar(&builder.rpcConf.BackendConfig.ExecutionClientTimeout,
			"execution-client-timeout",
			defaultConfig.rpcConf.BackendConfig.ExecutionClientTimeout,
			"grpc client timeout for an execution node")
		flags.UintVar(&builder.rpcConf.BackendConfig.ConnectionPoolSize,
			"connection-pool-size",
			defaultConfig.rpcConf.BackendConfig.ConnectionPoolSize,
			"maximum number of connections allowed in the connection pool, size of 0 disables the connection pooling, and anything less than the default size will be overridden to use the default size")
		flags.UintVar(&builder.rpcConf.MaxMsgSize,
			"rpc-max-message-size",
			grpcutils.DefaultMaxMsgSize,
			"the maximum message size in bytes for messages sent or received over grpc")
		flags.UintVar(&builder.rpcConf.BackendConfig.MaxHeightRange,
			"rpc-max-height-range",
			defaultConfig.rpcConf.BackendConfig.MaxHeightRange,
			"maximum size for height range requests")
		flags.StringSliceVar(&builder.rpcConf.BackendConfig.PreferredExecutionNodeIDs,
			"preferred-execution-node-ids",
			defaultConfig.rpcConf.BackendConfig.PreferredExecutionNodeIDs,
			"comma separated list of execution nodes ids to choose from when making an upstream call e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.StringSliceVar(&builder.rpcConf.BackendConfig.FixedExecutionNodeIDs,
			"fixed-execution-node-ids",
			defaultConfig.rpcConf.BackendConfig.FixedExecutionNodeIDs,
			"comma separated list of execution nodes ids to choose from when making an upstream call if no matching preferred execution id is found e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.StringVar(&builder.rpcConf.CompressorName,
			"grpc-compressor",
			defaultConfig.rpcConf.CompressorName,
			"name of grpc compressor that will be used for requests to other nodes. One of (gzip, snappy, deflate)")
		flags.BoolVar(&builder.logTxTimeToFinalized, "log-tx-time-to-finalized", defaultConfig.logTxTimeToFinalized, "log transaction time to finalized")
		flags.BoolVar(&builder.logTxTimeToExecuted, "log-tx-time-to-executed", defaultConfig.logTxTimeToExecuted, "log transaction time to executed")
		flags.BoolVar(&builder.logTxTimeToFinalizedExecuted,
			"log-tx-time-to-finalized-executed",
			defaultConfig.logTxTimeToFinalizedExecuted,
			"log transaction time to finalized and executed")
		flags.BoolVar(&builder.pingEnabled,
			"ping-enabled",
			defaultConfig.pingEnabled,
			"whether to enable the ping process that pings all other peers and report the connectivity to metrics")
		flags.BoolVar(&builder.retryEnabled, "retry-enabled", defaultConfig.retryEnabled, "whether to enable the retry mechanism at the access node level")
		flags.BoolVar(&builder.rpcMetricsEnabled, "rpc-metrics-enabled", defaultConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")
		flags.UintVar(&builder.TxResultCacheSize, "transaction-result-cache-size", defaultConfig.TxResultCacheSize, "transaction result cache size.(Disabled by default i.e 0)")
		flags.UintVar(&builder.TxErrorMessagesCacheSize, "transaction-error-messages-cache-size", defaultConfig.TxErrorMessagesCacheSize, "transaction error messages cache size.(By default 1000)")
		flags.StringVarP(&builder.nodeInfoFile,
			"node-info-file",
			"",
			defaultConfig.nodeInfoFile,
			"full path to a json file which provides more details about nodes when reporting its reachability metrics")
		flags.StringToIntVar(&builder.apiRatelimits, "api-rate-limits", defaultConfig.apiRatelimits, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&builder.apiBurstlimits, "api-burst-limits", defaultConfig.apiBurstlimits, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.BoolVar(&builder.supportsObserver, "supports-observer", defaultConfig.supportsObserver, "true if this staked access node supports observer or follower connections")
		flags.StringVar(&builder.PublicNetworkConfig.BindAddress, "public-network-address", defaultConfig.PublicNetworkConfig.BindAddress, "staked access node's public network bind address")
		flags.BoolVar(&builder.rpcConf.BackendConfig.CircuitBreakerConfig.Enabled,
			"circuit-breaker-enabled",
			defaultConfig.rpcConf.BackendConfig.CircuitBreakerConfig.Enabled,
			"specifies whether the circuit breaker is enabled for collection and execution API clients.")
		flags.DurationVar(&builder.rpcConf.BackendConfig.CircuitBreakerConfig.RestoreTimeout,
			"circuit-breaker-restore-timeout",
			defaultConfig.rpcConf.BackendConfig.CircuitBreakerConfig.RestoreTimeout,
			"duration after which the circuit breaker will restore the connection to the client after closing it due to failures. Default value is 60s")
		flags.Uint32Var(&builder.rpcConf.BackendConfig.CircuitBreakerConfig.MaxFailures,
			"circuit-breaker-max-failures",
			defaultConfig.rpcConf.BackendConfig.CircuitBreakerConfig.MaxFailures,
			"maximum number of failed calls to the client that will cause the circuit breaker to close the connection. Default value is 5")
		flags.Uint32Var(&builder.rpcConf.BackendConfig.CircuitBreakerConfig.MaxRequests,
			"circuit-breaker-max-requests",
			defaultConfig.rpcConf.BackendConfig.CircuitBreakerConfig.MaxRequests,
			"maximum number of requests to check if connection restored after timeout. Default value is 1")
		// ExecutionDataRequester config
		flags.BoolVar(&builder.executionDataSyncEnabled,
			"execution-data-sync-enabled",
			defaultConfig.executionDataSyncEnabled,
			"whether to enable the execution data sync protocol")
		flags.StringVar(&builder.executionDataDir, "execution-data-dir", defaultConfig.executionDataDir, "directory to use for Execution Data database")
		flags.Uint64Var(&builder.executionDataStartHeight,
			"execution-data-start-height",
			defaultConfig.executionDataStartHeight,
			"height of first block to sync execution data from when starting with an empty Execution Data database")
		flags.Uint64Var(&builder.executionDataConfig.MaxSearchAhead,
			"execution-data-max-search-ahead",
			defaultConfig.executionDataConfig.MaxSearchAhead,
			"max number of heights to search ahead of the lowest outstanding execution data height")
		flags.DurationVar(&builder.executionDataConfig.FetchTimeout,
			"execution-data-fetch-timeout",
			defaultConfig.executionDataConfig.FetchTimeout,
			"initial timeout to use when fetching execution data from the network. timeout increases using an incremental backoff until execution-data-max-fetch-timeout. e.g. 30s")
		flags.DurationVar(&builder.executionDataConfig.MaxFetchTimeout,
			"execution-data-max-fetch-timeout",
			defaultConfig.executionDataConfig.MaxFetchTimeout,
			"maximum timeout to use when fetching execution data from the network e.g. 300s")
		flags.DurationVar(&builder.executionDataConfig.RetryDelay,
			"execution-data-retry-delay",
			defaultConfig.executionDataConfig.RetryDelay,
			"initial delay for exponential backoff when fetching execution data fails e.g. 10s")
		flags.DurationVar(&builder.executionDataConfig.MaxRetryDelay,
			"execution-data-max-retry-delay",
			defaultConfig.executionDataConfig.MaxRetryDelay,
			"maximum delay for exponential backoff when fetching execution data fails e.g. 5m")

		// Execution State Streaming API
		flags.Uint32Var(&builder.stateStreamConf.ExecutionDataCacheSize, "execution-data-cache-size", defaultConfig.stateStreamConf.ExecutionDataCacheSize, "block execution data cache size")
		flags.Uint32Var(&builder.stateStreamConf.MaxGlobalStreams, "state-stream-global-max-streams", defaultConfig.stateStreamConf.MaxGlobalStreams, "global maximum number of concurrent streams")
		flags.UintVar(&builder.stateStreamConf.MaxExecutionDataMsgSize,
			"state-stream-max-message-size",
			defaultConfig.stateStreamConf.MaxExecutionDataMsgSize,
			"maximum size for a gRPC message containing block execution data")
		flags.StringToIntVar(&builder.stateStreamFilterConf,
			"state-stream-event-filter-limits",
			defaultConfig.stateStreamFilterConf,
			"event filter limits for ExecutionData SubscribeEvents API e.g. EventTypes=100,Addresses=100,Contracts=100 etc.")
		flags.DurationVar(&builder.stateStreamConf.ClientSendTimeout,
			"state-stream-send-timeout",
			defaultConfig.stateStreamConf.ClientSendTimeout,
			"maximum wait before timing out while sending a response to a streaming client e.g. 30s")
		flags.UintVar(&builder.stateStreamConf.ClientSendBufferSize,
			"state-stream-send-buffer-size",
			defaultConfig.stateStreamConf.ClientSendBufferSize,
			"maximum number of responses to buffer within a stream")
		flags.Float64Var(&builder.stateStreamConf.ResponseLimit,
			"state-stream-response-limit",
			defaultConfig.stateStreamConf.ResponseLimit,
			"max number of responses per second to send over streaming endpoints. this helps manage resources consumed by each client querying data not in the cache e.g. 3 or 0.5. 0 means no limit")
		flags.Uint64Var(&builder.stateStreamConf.HeartbeatInterval,
			"state-stream-heartbeat-interval",
			defaultConfig.stateStreamConf.HeartbeatInterval,
			"default interval in blocks at which heartbeat messages should be sent. applied when client did not specify a value.")
		flags.Uint32Var(&builder.stateStreamConf.RegisterIDsRequestLimit,
			"state-stream-max-register-values",
			defaultConfig.stateStreamConf.RegisterIDsRequestLimit,
			"maximum number of register ids to include in a single request to the GetRegisters endpoint")

		// Execution Data Indexer
		flags.BoolVar(&builder.executionDataIndexingEnabled,
			"execution-data-indexing-enabled",
			defaultConfig.executionDataIndexingEnabled,
			"whether to enable the execution data indexing")
		flags.StringVar(&builder.registersDBPath, "execution-state-dir", defaultConfig.registersDBPath, "directory to use for execution-state database")
		flags.StringVar(&builder.checkpointFile, "execution-state-checkpoint", defaultConfig.checkpointFile, "execution-state checkpoint file")

		// Script Execution
		flags.StringVar(&builder.rpcConf.BackendConfig.ScriptExecutionMode,
			"script-execution-mode",
			defaultConfig.rpcConf.BackendConfig.ScriptExecutionMode,
			"mode to use when executing scripts. one of (local-only, execution-nodes-only, failover, compare)")
		flags.Uint64Var(&builder.scriptExecutorConfig.ComputationLimit,
			"script-execution-computation-limit",
			defaultConfig.scriptExecutorConfig.ComputationLimit,
			"maximum number of computation units a locally executed script can use. default: 100000")
		flags.IntVar(&builder.scriptExecutorConfig.MaxErrorMessageSize,
			"script-execution-max-error-length",
			defaultConfig.scriptExecutorConfig.MaxErrorMessageSize,
			"maximum number characters to include in error message strings. additional characters are truncated. default: 1000")
		flags.DurationVar(&builder.scriptExecutorConfig.LogTimeThreshold,
			"script-execution-log-time-threshold",
			defaultConfig.scriptExecutorConfig.LogTimeThreshold,
			"emit a log for any scripts that take over this threshold. default: 1s")
		flags.DurationVar(&builder.scriptExecutorConfig.ExecutionTimeLimit,
			"script-execution-timeout",
			defaultConfig.scriptExecutorConfig.ExecutionTimeLimit,
			"timeout value for locally executed scripts. default: 10s")

	}).ValidateFlags(func() error {
		if builder.supportsObserver && (builder.PublicNetworkConfig.BindAddress == cmd.NotSet || builder.PublicNetworkConfig.BindAddress == "") {
			return errors.New("public-network-address must be set if supports-observer is true")
		}
		if builder.executionDataSyncEnabled {
			if builder.executionDataConfig.FetchTimeout <= 0 {
				return errors.New("execution-data-fetch-timeout must be greater than 0")
			}
			if builder.executionDataConfig.MaxFetchTimeout < builder.executionDataConfig.FetchTimeout {
				return errors.New("execution-data-max-fetch-timeout must be greater than execution-data-fetch-timeout")
			}
			if builder.executionDataConfig.RetryDelay <= 0 {
				return errors.New("execution-data-retry-delay must be greater than 0")
			}
			if builder.executionDataConfig.MaxRetryDelay < builder.executionDataConfig.RetryDelay {
				return errors.New("execution-data-max-retry-delay must be greater than or equal to execution-data-retry-delay")
			}
			if builder.executionDataConfig.MaxSearchAhead == 0 {
				return errors.New("execution-data-max-search-ahead must be greater than 0")
			}
		}
		if builder.stateStreamConf.ListenAddr != "" {
			if builder.stateStreamConf.ExecutionDataCacheSize == 0 {
				return errors.New("execution-data-cache-size must be greater than 0")
			}
			if builder.stateStreamConf.ClientSendBufferSize == 0 {
				return errors.New("state-stream-send-buffer-size must be greater than 0")
			}
			if len(builder.stateStreamFilterConf) > 3 {
				return errors.New("state-stream-event-filter-limits must have at most 3 keys (EventTypes, Addresses, Contracts)")
			}
			for key, value := range builder.stateStreamFilterConf {
				switch key {
				case "EventTypes", "Addresses", "Contracts":
					if value <= 0 {
						return fmt.Errorf("state-stream-event-filter-limits %s must be greater than 0", key)
					}
				default:
					return errors.New("state-stream-event-filter-limits may only contain the keys EventTypes, Addresses, Contracts")
				}
			}
			if builder.stateStreamConf.ResponseLimit < 0 {
				return errors.New("state-stream-response-limit must be greater than or equal to 0")
			}
			if builder.stateStreamConf.RegisterIDsRequestLimit <= 0 {
				return errors.New("state-stream-max-register-values must be greater than 0")
			}
		}
		if builder.rpcConf.BackendConfig.CircuitBreakerConfig.Enabled {
			if builder.rpcConf.BackendConfig.CircuitBreakerConfig.MaxFailures == 0 {
				return errors.New("circuit-breaker-max-failures must be greater than 0")
			}
			if builder.rpcConf.BackendConfig.CircuitBreakerConfig.MaxRequests == 0 {
				return errors.New("circuit-breaker-max-requests must be greater than 0")
			}
			if builder.rpcConf.BackendConfig.CircuitBreakerConfig.RestoreTimeout <= 0 {
				return errors.New("circuit-breaker-restore-timeout must be greater than 0")
			}
		}
		if builder.TxErrorMessagesCacheSize == 0 {
			return errors.New("transaction-error-messages-cache-size must be greater than 0")
		}

		return nil
	})
}

func publicNetworkMsgValidators(log zerolog.Logger, idProvider module.IdentityProvider, selfID flow.Identifier) []network.MessageValidator {
	return []network.MessageValidator{
		// filter out messages sent by this node itself
		validator.ValidateNotSender(selfID),
		validator.NewAnyValidator(
			// message should be either from a valid staked node
			validator.NewOriginValidator(
				id.NewIdentityFilterIdentifierProvider(filter.IsValidCurrentEpochParticipant, idProvider),
			),
			// or the message should be specifically targeted for this node
			validator.ValidateTarget(log, selfID),
		),
	}
}

func (builder *FlowAccessNodeBuilder) InitIDProviders() {
	builder.Module("id providers", func(node *cmd.NodeConfig) error {
		idCache, err := cache.NewProtocolStateIDCache(node.Logger, node.State, node.ProtocolEvents)
		if err != nil {
			return fmt.Errorf("could not initialize ProtocolStateIDCache: %w", err)
		}
		builder.IDTranslator = translator.NewHierarchicalIDTranslator(idCache, translator.NewPublicNetworkIDTranslator())

		// The following wrapper allows to disallow-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of disallow-listed nodes to true
		disallowListWrapper, err := cache.NewNodeDisallowListWrapper(idCache, node.DB, func() network.DisallowListNotificationConsumer {
			return builder.NetworkUnderlay
		})
		if err != nil {
			return fmt.Errorf("could not initialize NodeBlockListWrapper: %w", err)
		}
		builder.IdentityProvider = disallowListWrapper

		// register the wrapper for dynamic configuration via admin command
		err = node.ConfigManager.RegisterIdentifierListConfig("network-id-provider-blocklist",
			disallowListWrapper.GetDisallowList, disallowListWrapper.Update)
		if err != nil {
			return fmt.Errorf("failed to register disallow-list wrapper with config manager: %w", err)
		}

		builder.SyncEngineParticipantsProviderFactory = func() module.IdentifierProvider {
			return id.NewIdentityFilterIdentifierProvider(
				filter.And(
					filter.HasRole(flow.RoleConsensus),
					filter.Not(filter.HasNodeID(node.Me.NodeID())),
					underlay.NotEjectedFilter,
				),
				builder.IdentityProvider,
			)
		}
		return nil
	})
}

func (builder *FlowAccessNodeBuilder) Initialize() error {
	builder.InitIDProviders()

	builder.EnqueueResolver()

	// enqueue the regular network
	builder.EnqueueNetworkInit()

	builder.AdminCommand("get-transactions", func(conf *cmd.NodeConfig) commands.AdminCommand {
		return storageCommands.NewGetTransactionsCommand(conf.State, conf.Storage.Payloads, conf.Storage.Collections)
	})

	// if this is an access node that supports public followers, enqueue the public network
	if builder.supportsObserver {
		builder.enqueuePublicNetworkInit()
		builder.enqueueRelayNetwork()
	}

	builder.EnqueuePingService()

	builder.EnqueueMetricsServerInit()

	if err := builder.RegisterBadgerMetrics(); err != nil {
		return err
	}

	builder.EnqueueTracer()
	builder.PreInit(cmd.DynamicStartPreInit)
	builder.ValidateRootSnapshot(badgerState.ValidRootSnapshotContainsEntityExpiryRange)

	return nil
}

func (builder *FlowAccessNodeBuilder) enqueueRelayNetwork() {
	builder.Component("relay network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		relayNet := relaynet.NewRelayNetwork(
			node.EngineRegistry,
			builder.AccessNodeConfig.PublicNetworkConfig.Network,
			node.Logger,
			map[channels.Channel]channels.Channel{
				channels.ReceiveBlocks: channels.PublicReceiveBlocks,
			},
		)
		node.EngineRegistry = relayNet
		return relayNet, nil
	})
}

func (builder *FlowAccessNodeBuilder) Build() (cmd.Node, error) {
	if builder.executionDataSyncEnabled {
		builder.BuildExecutionSyncComponents()
	}

	builder.
		BuildConsensusFollower().
		Module("collection node client", func(node *cmd.NodeConfig) error {
			// collection node address is optional (if not specified, collection nodes will be chosen at random)
			if strings.TrimSpace(builder.rpcConf.CollectionAddr) == "" {
				node.Logger.Info().Msg("using a dynamic collection node address")
				return nil
			}

			node.Logger.Info().
				Str("collection_node", builder.rpcConf.CollectionAddr).
				Msg("using the static collection node address")

			collectionRPCConn, err := grpc.Dial(
				builder.rpcConf.CollectionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(builder.rpcConf.MaxMsgSize))),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				rpcConnection.WithClientTimeoutOption(builder.rpcConf.BackendConfig.CollectionClientTimeout))
			if err != nil {
				return err
			}
			builder.CollectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("historical access node clients", func(node *cmd.NodeConfig) error {
			addrs := strings.Split(builder.rpcConf.HistoricalAccessAddrs, ",")
			for _, addr := range addrs {
				if strings.TrimSpace(addr) == "" {
					continue
				}
				node.Logger.Info().Str("access_nodes", addr).Msg("historical access node addresses")

				historicalAccessRPCConn, err := grpc.Dial(
					addr,
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(builder.rpcConf.MaxMsgSize))),
					grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return err
				}
				builder.HistoricalAccessRPCs = append(builder.HistoricalAccessRPCs, access.NewAccessAPIClient(historicalAccessRPCConn))
			}
			return nil
		}).
		Module("transaction timing mempools", func(node *cmd.NodeConfig) error {
			var err error
			builder.TransactionTimings, err = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
			if err != nil {
				return err
			}

			builder.CollectionsToMarkFinalized, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			builder.CollectionsToMarkExecuted, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			builder.BlocksToMarkExecuted, err = stdmap.NewTimes(1 * 300) // assume 1 block per second * 300 seconds
			return err
		}).
		Module("transaction metrics", func(node *cmd.NodeConfig) error {
			builder.TransactionMetrics = metrics.NewTransactionCollector(
				node.Logger,
				builder.TransactionTimings,
				builder.logTxTimeToFinalized,
				builder.logTxTimeToExecuted,
				builder.logTxTimeToFinalizedExecuted,
			)
			return nil
		}).
		Module("rest metrics", func(node *cmd.NodeConfig) error {
			m, err := metrics.NewRestCollector(routes.URLToRoute, node.MetricsRegisterer)
			if err != nil {
				return err
			}
			builder.RestMetrics = m
			return nil
		}).
		Module("access metrics", func(node *cmd.NodeConfig) error {
			builder.AccessMetrics = metrics.NewAccessCollector(
				metrics.WithTransactionMetrics(builder.TransactionMetrics),
				metrics.WithBackendScriptsMetrics(builder.TransactionMetrics),
				metrics.WithRestMetrics(builder.RestMetrics),
			)
			return nil
		}).
		Module("ping metrics", func(node *cmd.NodeConfig) error {
			builder.PingMetrics = metrics.NewPingCollector()
			return nil
		}).
		Module("server certificate", func(node *cmd.NodeConfig) error {
			// generate the server certificate that will be served by the GRPC server
			x509Certificate, err := grpcutils.X509Certificate(node.NetworkKey)
			if err != nil {
				return err
			}
			tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
			builder.rpcConf.TransportCredentials = credentials.NewTLS(tlsConfig)
			return nil
		}).
		Module("creating grpc servers", func(node *cmd.NodeConfig) error {
			builder.secureGrpcServer = grpcserver.NewGrpcServerBuilder(
				node.Logger,
				builder.rpcConf.SecureGRPCListenAddr,
				builder.rpcConf.MaxMsgSize,
				builder.rpcMetricsEnabled,
				builder.apiRatelimits,
				builder.apiBurstlimits,
				grpcserver.WithTransportCredentials(builder.rpcConf.TransportCredentials)).Build()

			builder.stateStreamGrpcServer = grpcserver.NewGrpcServerBuilder(
				node.Logger,
				builder.stateStreamConf.ListenAddr,
				builder.stateStreamConf.MaxExecutionDataMsgSize,
				builder.rpcMetricsEnabled,
				builder.apiRatelimits,
				builder.apiBurstlimits,
				grpcserver.WithStreamInterceptor()).Build()

			if builder.rpcConf.UnsecureGRPCListenAddr != builder.stateStreamConf.ListenAddr {
				builder.unsecureGrpcServer = grpcserver.NewGrpcServerBuilder(node.Logger,
					builder.rpcConf.UnsecureGRPCListenAddr,
					builder.rpcConf.MaxMsgSize,
					builder.rpcMetricsEnabled,
					builder.apiRatelimits,
					builder.apiBurstlimits).Build()
			} else {
				builder.unsecureGrpcServer = builder.stateStreamGrpcServer
			}

			return nil
		}).
		Module("backend script executor", func(node *cmd.NodeConfig) error {
			builder.ScriptExecutor = backend.NewScriptExecutor()
			return nil
		}).
		Module("async register store", func(node *cmd.NodeConfig) error {
			builder.RegistersAsyncStore = execution.NewRegistersAsyncStore()
			return nil
		}).
		Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			config := builder.rpcConf
			backendConfig := config.BackendConfig
			accessMetrics := builder.AccessMetrics
			cacheSize := int(backendConfig.ConnectionPoolSize)

			var connBackendCache *rpcConnection.Cache
			if cacheSize > 0 {
				backendCache, err := backend.NewCache(node.Logger, accessMetrics, cacheSize)
				if err != nil {
					return nil, fmt.Errorf("could not initialize backend cache: %w", err)
				}
				connBackendCache = rpcConnection.NewCache(backendCache, cacheSize)
			}

			connFactory := &rpcConnection.ConnectionFactoryImpl{
				CollectionGRPCPort:        builder.collectionGRPCPort,
				ExecutionGRPCPort:         builder.executionGRPCPort,
				CollectionNodeGRPCTimeout: backendConfig.CollectionClientTimeout,
				ExecutionNodeGRPCTimeout:  backendConfig.ExecutionClientTimeout,
				AccessMetrics:             accessMetrics,
				Log:                       node.Logger,
				Manager: rpcConnection.NewManager(
					connBackendCache,
					node.Logger,
					accessMetrics,
					config.MaxMsgSize,
					backendConfig.CircuitBreakerConfig,
					config.CompressorName,
				),
			}

			scriptExecMode, err := backend.ParseScriptExecutionMode(config.BackendConfig.ScriptExecutionMode)
			if err != nil {
				return nil, fmt.Errorf("could not parse script execution mode: %w", err)
			}

			nodeBackend, err := backend.New(backend.Params{
				State:                     node.State,
				CollectionRPC:             builder.CollectionRPC,
				HistoricalAccessNodes:     builder.HistoricalAccessRPCs,
				Blocks:                    node.Storage.Blocks,
				Headers:                   node.Storage.Headers,
				Collections:               node.Storage.Collections,
				Transactions:              node.Storage.Transactions,
				ExecutionReceipts:         node.Storage.Receipts,
				ExecutionResults:          node.Storage.Results,
				ChainID:                   node.RootChainID,
				AccessMetrics:             builder.AccessMetrics,
				ConnFactory:               connFactory,
				RetryEnabled:              builder.retryEnabled,
				MaxHeightRange:            backendConfig.MaxHeightRange,
				PreferredExecutionNodeIDs: backendConfig.PreferredExecutionNodeIDs,
				FixedExecutionNodeIDs:     backendConfig.FixedExecutionNodeIDs,
				Log:                       node.Logger,
				SnapshotHistoryLimit:      backend.DefaultSnapshotHistoryLimit,
				Communicator:              backend.NewNodeCommunicator(backendConfig.CircuitBreakerConfig.Enabled),
				TxResultCacheSize:         builder.TxResultCacheSize,
				TxErrorMessagesCacheSize:  builder.TxErrorMessagesCacheSize,
				ScriptExecutor:            builder.ScriptExecutor,
				ScriptExecutionMode:       scriptExecMode,
			})
			if err != nil {
				return nil, fmt.Errorf("could not initialize backend: %w", err)
			}

			engineBuilder, err := rpc.NewBuilder(
				node.Logger,
				node.State,
				config,
				node.RootChainID,
				builder.AccessMetrics,
				builder.rpcMetricsEnabled,
				builder.Me,
				nodeBackend,
				nodeBackend,
				builder.secureGrpcServer,
				builder.unsecureGrpcServer,
				builder.stateStreamBackend,
				builder.stateStreamConf,
			)
			if err != nil {
				return nil, err
			}

			builder.RpcEng, err = engineBuilder.
				WithLegacy().
				WithBlockSignerDecoder(signature.NewBlockSignerDecoder(builder.Committee)).
				Build()
			if err != nil {
				return nil, err
			}
			builder.FollowerDistributor.AddOnBlockFinalizedConsumer(builder.RpcEng.OnFinalizedBlock)

			return builder.RpcEng, nil
		}).
		Component("ingestion engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error

			builder.RequestEng, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.EngineRegistry,
				node.Me,
				node.State,
				channels.RequestCollections,
				filter.HasRole(flow.RoleCollection),
				func() flow.Entity { return &flow.Collection{} },
			)
			if err != nil {
				return nil, fmt.Errorf("could not create requester engine: %w", err)
			}

			builder.IngestEng, err = ingestion.New(
				node.Logger,
				node.EngineRegistry,
				node.State,
				node.Me,
				builder.RequestEng,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Results,
				node.Storage.Receipts,
				builder.AccessMetrics,
				builder.CollectionsToMarkFinalized,
				builder.CollectionsToMarkExecuted,
				builder.BlocksToMarkExecuted,
			)
			if err != nil {
				return nil, err
			}
			builder.RequestEng.WithHandle(builder.IngestEng.OnCollection)
			builder.FollowerDistributor.AddOnBlockFinalizedConsumer(builder.IngestEng.OnFinalizedBlock)

			return builder.IngestEng, nil
		}).
		Component("requester engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return builder.RequestEng, nil
		})

	if builder.supportsObserver {
		builder.Component("public sync request handler", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			syncRequestHandler, err := synceng.NewRequestHandlerEngine(
				node.Logger.With().Bool("public", true).Logger(),
				unstaked.NewUnstakedEngineCollector(node.Metrics.Engine),
				builder.AccessNodeConfig.PublicNetworkConfig.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				builder.SyncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create public sync request handler: %w", err)
			}
			builder.FollowerDistributor.AddFinalizationConsumer(syncRequestHandler)

			return syncRequestHandler, nil
		})
	}

	builder.Component("secure grpc server", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return builder.secureGrpcServer, nil
	})

	builder.Component("state stream unsecure grpc server", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return builder.stateStreamGrpcServer, nil
	})

	if builder.rpcConf.UnsecureGRPCListenAddr != builder.stateStreamConf.ListenAddr {
		builder.Component("unsecure grpc server", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			return builder.unsecureGrpcServer, nil
		})
	}

	builder.Component("ping engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		ping, err := pingeng.New(
			node.Logger,
			node.IdentityProvider,
			node.IDTranslator,
			node.Me,
			builder.PingMetrics,
			builder.pingEnabled,
			builder.nodeInfoFile,
			node.PingService,
		)

		if err != nil {
			return nil, fmt.Errorf("could not create ping engine: %w", err)
		}

		return ping, nil
	})

	return builder.FlowNodeBuilder.Build()
}

// enqueuePublicNetworkInit enqueues the public network component initialized for the staked node
func (builder *FlowAccessNodeBuilder) enqueuePublicNetworkInit() {
	var publicLibp2pNode p2p.LibP2PNode
	builder.
		Module("public network metrics", func(node *cmd.NodeConfig) error {
			builder.PublicNetworkConfig.Metrics = metrics.NewNetworkCollector(builder.Logger, metrics.WithNetworkPrefix("public"))
			return nil
		}).
		Component("public libp2p node", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error
			publicLibp2pNode, err = builder.initPublicLibp2pNode(
				builder.NodeConfig.NetworkKey,
				builder.PublicNetworkConfig.BindAddress,
				builder.PublicNetworkConfig.Metrics)
			if err != nil {
				return nil, fmt.Errorf("could not create public libp2p node: %w", err)
			}

			return publicLibp2pNode, nil
		}).
		Component("public network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			msgValidators := publicNetworkMsgValidators(node.Logger.With().Bool("public", true).Logger(), node.IdentityProvider, builder.NodeID)
			receiveCache := netcache.NewHeroReceiveCache(builder.FlowConfig.NetworkConfig.NetworkReceivedMessageCacheSize,
				builder.Logger,
				metrics.NetworkReceiveCacheMetricsFactory(builder.HeroCacheMetricsFactory(), network.PublicNetwork))

			err := node.Metrics.Mempool.Register(metrics.PrependPublicPrefix(metrics.ResourceNetworkingReceiveCache), receiveCache.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register networking receive cache metric: %w", err)
			}

			net, err := underlay.NewNetwork(&underlay.NetworkConfig{
				Logger:                builder.Logger.With().Str("module", "public-network").Logger(),
				Libp2pNode:            publicLibp2pNode,
				Codec:                 cborcodec.NewCodec(),
				Me:                    builder.Me,
				Topology:              topology.EmptyTopology{}, // topology returns empty list since peers are not known upfront
				Metrics:               builder.PublicNetworkConfig.Metrics,
				BitSwapMetrics:        builder.Metrics.Bitswap,
				IdentityProvider:      builder.IdentityProvider,
				ReceiveCache:          receiveCache,
				ConduitFactory:        conduit.NewDefaultConduitFactory(),
				SporkId:               builder.SporkID,
				UnicastMessageTimeout: underlay.DefaultUnicastTimeout,
				IdentityTranslator:    builder.IDTranslator,
				AlspCfg: &alspmgr.MisbehaviorReportManagerConfig{
					Logger:                  builder.Logger,
					SpamRecordCacheSize:     builder.FlowConfig.NetworkConfig.AlspConfig.SpamRecordCacheSize,
					SpamReportQueueSize:     builder.FlowConfig.NetworkConfig.AlspConfig.SpamReportQueueSize,
					DisablePenalty:          builder.FlowConfig.NetworkConfig.AlspConfig.DisablePenalty,
					HeartBeatInterval:       builder.FlowConfig.NetworkConfig.AlspConfig.HearBeatInterval,
					AlspMetrics:             builder.Metrics.Network,
					NetworkType:             network.PublicNetwork,
					HeroCacheMetricsFactory: builder.HeroCacheMetricsFactory(),
				},
				SlashingViolationConsumerFactory: func(adapter network.ConduitAdapter) network.ViolationsConsumer {
					return slashing.NewSlashingViolationsConsumer(builder.Logger, builder.Metrics.Network, adapter)
				},
			}, underlay.WithMessageValidators(msgValidators...))
			if err != nil {
				return nil, fmt.Errorf("could not initialize network: %w", err)
			}

			builder.NetworkUnderlay = net
			builder.AccessNodeConfig.PublicNetworkConfig.Network = net

			node.Logger.Info().Msgf("network will run on address: %s", builder.PublicNetworkConfig.BindAddress)
			return net, nil
		}).
		Component("public peer manager", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			return publicLibp2pNode.PeerManagerComponent(), nil
		})
}

// initPublicLibp2pNode initializes the public libp2p node for the public (unstaked) network.
// The LibP2P host is created with the following options:
//   - DHT as server
//   - The address from the node config or the specified bind address as the listen address
//   - The passed in private key as the libp2p key
//   - No connection gater
//   - Default Flow libp2p pubsub options
//
// Args:
//   - networkKey: The private key to use for the libp2p node
//
// - bindAddress: The address to bind the libp2p node to.
// - networkMetrics: The metrics collector for the network
// Returns:
// - The libp2p node instance for the public network.
// - Any error encountered during initialization. Any error should be considered fatal.
func (builder *FlowAccessNodeBuilder) initPublicLibp2pNode(networkKey crypto.PrivateKey, bindAddress string, networkMetrics module.LibP2PMetrics) (p2p.LibP2PNode,
	error) {
	connManager, err := connection.NewConnManager(builder.Logger, networkMetrics, &builder.FlowConfig.NetworkConfig.ConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("could not create connection manager: %w", err)
	}

	libp2pNode, err := p2pbuilder.NewNodeBuilder(builder.Logger, &builder.FlowConfig.NetworkConfig.GossipSub, &p2pbuilderconfig.MetricsConfig{
		HeroCacheFactory: builder.HeroCacheMetricsFactory(),
		Metrics:          networkMetrics,
	},
		network.PublicNetwork,
		bindAddress,
		networkKey,
		builder.SporkID,
		builder.IdentityProvider,
		&builder.FlowConfig.NetworkConfig.ResourceManager,
		&p2pbuilderconfig.PeerManagerConfig{
			// TODO: eventually, we need pruning enabled even on public network. However, it needs a modified version of
			// the peer manager that also operate on the public identities.
			ConnectionPruning: connection.PruningDisabled,
			UpdateInterval:    builder.FlowConfig.NetworkConfig.PeerUpdateInterval,
			ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
		},
		&p2p.DisallowListCacheConfig{
			MaxSize: builder.FlowConfig.NetworkConfig.DisallowListNotificationCacheSize,
			Metrics: metrics.DisallowListCacheMetricsFactory(builder.HeroCacheMetricsFactory(), network.PublicNetwork),
		},
		&p2pbuilderconfig.UnicastConfig{
			Unicast: builder.FlowConfig.NetworkConfig.Unicast,
		}).
		SetBasicResolver(builder.Resolver).
		SetSubscriptionFilter(subscription.NewRoleBasedFilter(flow.RoleAccess, builder.IdentityProvider)).
		SetConnectionManager(connManager).
		SetRoutingSystem(func(ctx context.Context, h host.Host) (routing.Routing, error) {
			return dht.NewDHT(ctx, h, protocols.FlowPublicDHTProtocolID(builder.SporkID), builder.Logger, networkMetrics, dht.AsServer())
		}).
		Build()

	if err != nil {
		return nil, fmt.Errorf("could not build libp2p node for staked access node: %w", err)
	}

	return libp2pNode, nil
}
