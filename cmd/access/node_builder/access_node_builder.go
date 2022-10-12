package node_builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/go-bitswap"
	badger "github.com/ipfs/go-ds-badger2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/admin/commands"
	storageCommands "github.com/onflow/flow-go/admin/commands/storage"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	consensuspubsub "github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/access/ingestion"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/follower"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/unstaked"
	"github.com/onflow/flow-go/module/state_synchronization"
	edrequester "github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast"
	relaynet "github.com/onflow/flow-go/network/relay"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
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
	baseOptions                  []cmd.Option

	PublicNetworkConfig PublicNetworkConfig
}

type PublicNetworkConfig struct {
	// NetworkKey crypto.PublicKey // TODO: do we need a different key for the public network?
	BindAddress string
	Network     network.Network
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
			UnsecureGRPCListenAddr:    "0.0.0.0:9000",
			SecureGRPCListenAddr:      "0.0.0.0:9001",
			HTTPListenAddr:            "0.0.0.0:8000",
			RESTListenAddr:            "",
			CollectionAddr:            "",
			HistoricalAccessAddrs:     "",
			CollectionClientTimeout:   3 * time.Second,
			ExecutionClientTimeout:    3 * time.Second,
			ConnectionPoolSize:        backend.DefaultConnectionPoolSize,
			MaxHeightRange:            backend.DefaultMaxHeightRange,
			PreferredExecutionNodeIDs: nil,
			FixedExecutionNodeIDs:     nil,
		},
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
		PublicNetworkConfig: PublicNetworkConfig{
			BindAddress: cmd.NotSet,
			Metrics:     metrics.NewNoopCollector(),
		},
		executionDataSyncEnabled: false,
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
	}
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow access node
// It is composed of the FlowNodeBuilder, the AccessNodeConfig and contains all the components and modules needed for the
// access nodes
type FlowAccessNodeBuilder struct {
	*cmd.FlowNodeBuilder
	*AccessNodeConfig

	// components
	LibP2PNode                 *p2pnode.Node
	FollowerState              protocol.MutableState
	SyncCore                   *chainsync.Core
	RpcEng                     *rpc.Engine
	FinalizationDistributor    *consensuspubsub.FinalizationDistributor
	FinalizedHeader            *synceng.FinalizedHeaderCache
	CollectionRPC              access.AccessAPIClient
	TransactionTimings         *stdmap.TransactionTimings
	CollectionsToMarkFinalized *stdmap.Times
	CollectionsToMarkExecuted  *stdmap.Times
	BlocksToMarkExecuted       *stdmap.Times
	TransactionMetrics         module.TransactionMetrics
	AccessMetrics              module.AccessMetrics
	PingMetrics                module.PingMetrics
	Committee                  hotstuff.Committee
	Finalized                  *flow.Header
	Pending                    []*flow.Header
	FollowerCore               module.HotStuffFollower
	ExecutionDataDownloader    execution_data.Downloader
	ExecutionDataRequester     state_synchronization.ExecutionDataRequester

	// The sync engine participants provider is the libp2p peer store for the access node
	// which is not available until after the network has started.
	// Hence, a factory function that needs to be called just before creating the sync engine
	SyncEngineParticipantsProviderFactory func() module.IdentifierProvider

	// engines
	IngestEng   *ingestion.Engine
	RequestEng  *requester.Engine
	FollowerEng *followereng.Engine
	SyncEng     *synceng.Engine
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
			state,
			node.Storage.Index,
			node.Storage.Payloads,
			node.Tracer,
			node.ProtocolEvents,
			blocktimer.DefaultBlockTimer,
		)
		builder.FollowerState = followerState

		return err
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildSyncCore() *FlowAccessNodeBuilder {
	builder.Module("sync core", func(node *cmd.NodeConfig) error {
		syncCore, err := chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector())
		builder.SyncCore = syncCore

		return err
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildCommittee() *FlowAccessNodeBuilder {
	builder.Module("committee", func(node *cmd.NodeConfig) error {
		// initialize consensus committee's membership state
		// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
		// Note: node.Me.NodeID() is not part of the consensus committee
		committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
		builder.Committee = committee

		return err
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

		followerCore, err := consensus.NewFollower(
			node.Logger,
			builder.Committee,
			node.Storage.Headers,
			final,
			verifier,
			builder.FinalizationDistributor,
			node.RootBlock.Header,
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
		// initialize cleaner for DB
		cleaner := bstorage.NewCleaner(node.Logger, node.DB, builder.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)
		conCache := buffer.NewPendingBlocks()

		followerEng, err := follower.New(
			node.Logger,
			node.Network,
			node.Me,
			node.Metrics.Engine,
			node.Metrics.Mempool,
			cleaner,
			node.Storage.Headers,
			node.Storage.Payloads,
			builder.FollowerState,
			conCache,
			builder.FollowerCore,
			builder.SyncCore,
			node.Tracer,
			follower.WithComplianceOptions(compliance.WithSkipNewProposalsThreshold(builder.ComplianceConfig.SkipNewProposalsThreshold)),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}
		builder.FollowerEng = followerEng

		return builder.FollowerEng, nil
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildFinalizedHeader() *FlowAccessNodeBuilder {
	builder.Component("finalized snapshot", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		finalizedHeader, err := synceng.NewFinalizedHeaderCache(node.Logger, node.State, builder.FinalizationDistributor)
		if err != nil {
			return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
		}
		builder.FinalizedHeader = finalizedHeader

		return builder.FinalizedHeader, nil
	})

	return builder
}

func (builder *FlowAccessNodeBuilder) buildSyncEngine() *FlowAccessNodeBuilder {
	builder.Component("sync engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		sync, err := synceng.New(
			node.Logger,
			node.Metrics.Engine,
			node.Network,
			node.Me,
			node.Storage.Blocks,
			builder.FollowerEng,
			builder.SyncCore,
			builder.FinalizedHeader,
			builder.SyncEngineParticipantsProviderFactory(),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create synchronization engine: %w", err)
		}
		builder.SyncEng = sync

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
		buildFinalizedHeader().
		buildSyncEngine()

	return builder
}

func (builder *FlowAccessNodeBuilder) BuildExecutionDataRequester() *FlowAccessNodeBuilder {
	var ds *badger.Datastore
	var bs network.BlobService
	var processedBlockHeight storage.ConsumerProgress
	var processedNotifications storage.ConsumerProgress
	var bsDependable *module.ProxiedReadyDoneAware

	builder.
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
			// uses the datastore's DB
			processedBlockHeight = bstorage.NewConsumerProgress(ds.DB, module.ConsumeProgressExecutionDataRequesterBlockHeight)
			return nil
		}).
		Module("processed notifications consumer progress", func(node *cmd.NodeConfig) error {
			// uses the datastore's DB
			processedNotifications = bstorage.NewConsumerProgress(ds.DB, module.ConsumeProgressExecutionDataRequesterNotification)
			return nil
		}).
		Module("blobservice peer manager dependencies", func(node *cmd.NodeConfig) error {
			bsDependable = module.NewProxiedReadyDoneAware()
			builder.PeerManagerDependencies.Add(bsDependable)
			return nil
		}).
		Component("execution data service", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			opts := []network.BlobServiceOption{
				blob.WithBitswapOptions(
					// Only allow block requests from staked ANs
					bitswap.WithPeerBlockRequestFilter(
						blob.AuthorizedRequester(nil, builder.IdentityProvider, builder.Logger),
					),
				),
			}

			var err error
			bs, err = node.Network.RegisterBlobService(channels.ExecutionDataService, ds, opts...)
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
				if builder.executionDataStartHeight <= builder.RootBlock.Header.Height {
					return nil, fmt.Errorf(
						"execution data start block height (%d) must be greater than the root block height (%d)",
						builder.executionDataStartHeight, builder.RootBlock.Header.Height)
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
				builder.executionDataConfig.InitialBlockHeight = builder.RootBlock.Header.Height
			}

			builder.ExecutionDataRequester = edrequester.New(
				builder.Logger,
				metrics.NewExecutionDataRequesterCollector(),
				builder.ExecutionDataDownloader,
				processedBlockHeight,
				processedNotifications,
				builder.State,
				builder.Storage.Headers,
				builder.Storage.Results,
				builder.Storage.Seals,
				builder.executionDataConfig,
			)

			builder.FinalizationDistributor.AddOnBlockFinalizedConsumer(builder.ExecutionDataRequester.OnBlockFinalized)

			return builder.ExecutionDataRequester, nil
		})

	return builder
}

type Option func(*AccessNodeConfig)

func FlowAccessNode(opts ...Option) *FlowAccessNodeBuilder {
	config := DefaultAccessNodeConfig()
	for _, opt := range opts {
		opt(config)
	}

	return &FlowAccessNodeBuilder{
		AccessNodeConfig:        config,
		FlowNodeBuilder:         cmd.FlowNode(flow.RoleAccess.String(), config.baseOptions...),
		FinalizationDistributor: consensuspubsub.NewFinalizationDistributor(),
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
		flags.StringVarP(&builder.rpcConf.UnsecureGRPCListenAddr, "rpc-addr", "r", defaultConfig.rpcConf.UnsecureGRPCListenAddr, "the address the unsecured gRPC server listens on")
		flags.StringVar(&builder.rpcConf.SecureGRPCListenAddr, "secure-rpc-addr", defaultConfig.rpcConf.SecureGRPCListenAddr, "the address the secure gRPC server listens on")
		flags.StringVarP(&builder.rpcConf.HTTPListenAddr, "http-addr", "h", defaultConfig.rpcConf.HTTPListenAddr, "the address the http proxy server listens on")
		flags.StringVar(&builder.rpcConf.RESTListenAddr, "rest-addr", defaultConfig.rpcConf.RESTListenAddr, "the address the REST server listens on (if empty the REST server will not be started)")
		flags.StringVarP(&builder.rpcConf.CollectionAddr, "static-collection-ingress-addr", "", defaultConfig.rpcConf.CollectionAddr, "the address (of the collection node) to send transactions to")
		flags.StringVarP(&builder.ExecutionNodeAddress, "script-addr", "s", defaultConfig.ExecutionNodeAddress, "the address (of the execution node) forward the script to")
		flags.StringVarP(&builder.rpcConf.HistoricalAccessAddrs, "historical-access-addr", "", defaultConfig.rpcConf.HistoricalAccessAddrs, "comma separated rpc addresses for historical access nodes")
		flags.DurationVar(&builder.rpcConf.CollectionClientTimeout, "collection-client-timeout", defaultConfig.rpcConf.CollectionClientTimeout, "grpc client timeout for a collection node")
		flags.DurationVar(&builder.rpcConf.ExecutionClientTimeout, "execution-client-timeout", defaultConfig.rpcConf.ExecutionClientTimeout, "grpc client timeout for an execution node")
		flags.UintVar(&builder.rpcConf.ConnectionPoolSize, "connection-pool-size", defaultConfig.rpcConf.ConnectionPoolSize, "maximum number of connections allowed in the connection pool, size of 0 disables the connection pooling, and anything less than the default size will be overridden to use the default size")
		flags.UintVar(&builder.rpcConf.MaxHeightRange, "rpc-max-height-range", defaultConfig.rpcConf.MaxHeightRange, "maximum size for height range requests")
		flags.StringSliceVar(&builder.rpcConf.PreferredExecutionNodeIDs, "preferred-execution-node-ids", defaultConfig.rpcConf.PreferredExecutionNodeIDs, "comma separated list of execution nodes ids to choose from when making an upstream call e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.StringSliceVar(&builder.rpcConf.FixedExecutionNodeIDs, "fixed-execution-node-ids", defaultConfig.rpcConf.FixedExecutionNodeIDs, "comma separated list of execution nodes ids to choose from when making an upstream call if no matching preferred execution id is found e.g. b4a4dbdcd443d...,fb386a6a... etc.")
		flags.BoolVar(&builder.logTxTimeToFinalized, "log-tx-time-to-finalized", defaultConfig.logTxTimeToFinalized, "log transaction time to finalized")
		flags.BoolVar(&builder.logTxTimeToExecuted, "log-tx-time-to-executed", defaultConfig.logTxTimeToExecuted, "log transaction time to executed")
		flags.BoolVar(&builder.logTxTimeToFinalizedExecuted, "log-tx-time-to-finalized-executed", defaultConfig.logTxTimeToFinalizedExecuted, "log transaction time to finalized and executed")
		flags.BoolVar(&builder.pingEnabled, "ping-enabled", defaultConfig.pingEnabled, "whether to enable the ping process that pings all other peers and report the connectivity to metrics")
		flags.BoolVar(&builder.retryEnabled, "retry-enabled", defaultConfig.retryEnabled, "whether to enable the retry mechanism at the access node level")
		flags.BoolVar(&builder.rpcMetricsEnabled, "rpc-metrics-enabled", defaultConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")
		flags.StringVarP(&builder.nodeInfoFile, "node-info-file", "", defaultConfig.nodeInfoFile, "full path to a json file which provides more details about nodes when reporting its reachability metrics")
		flags.StringToIntVar(&builder.apiRatelimits, "api-rate-limits", defaultConfig.apiRatelimits, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&builder.apiBurstlimits, "api-burst-limits", defaultConfig.apiBurstlimits, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.BoolVar(&builder.supportsObserver, "supports-observer", defaultConfig.supportsObserver, "true if this staked access node supports observer or follower connections")
		flags.StringVar(&builder.PublicNetworkConfig.BindAddress, "public-network-address", defaultConfig.PublicNetworkConfig.BindAddress, "staked access node's public network bind address")

		// ExecutionDataRequester config
		flags.BoolVar(&builder.executionDataSyncEnabled, "execution-data-sync-enabled", defaultConfig.executionDataSyncEnabled, "whether to enable the execution data sync protocol")
		flags.StringVar(&builder.executionDataDir, "execution-data-dir", defaultConfig.executionDataDir, "directory to use for Execution Data database")
		flags.Uint64Var(&builder.executionDataStartHeight, "execution-data-start-height", defaultConfig.executionDataStartHeight, "height of first block to sync execution data from when starting with an empty Execution Data database")
		flags.Uint64Var(&builder.executionDataConfig.MaxSearchAhead, "execution-data-max-search-ahead", defaultConfig.executionDataConfig.MaxSearchAhead, "max number of heights to search ahead of the lowest outstanding execution data height")
		flags.DurationVar(&builder.executionDataConfig.FetchTimeout, "execution-data-fetch-timeout", defaultConfig.executionDataConfig.FetchTimeout, "initial timeout to use when fetching execution data from the network. timeout increases using an incremental backoff until execution-data-max-fetch-timeout. e.g. 30s")
		flags.DurationVar(&builder.executionDataConfig.MaxFetchTimeout, "execution-data-max-fetch-timeout", defaultConfig.executionDataConfig.MaxFetchTimeout, "maximum timeout to use when fetching execution data from the network e.g. 300s")
		flags.DurationVar(&builder.executionDataConfig.RetryDelay, "execution-data-retry-delay", defaultConfig.executionDataConfig.RetryDelay, "initial delay for exponential backoff when fetching execution data fails e.g. 10s")
		flags.DurationVar(&builder.executionDataConfig.MaxRetryDelay, "execution-data-max-retry-delay", defaultConfig.executionDataConfig.MaxRetryDelay, "maximum delay for exponential backoff when fetching execution data fails e.g. 5m")
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

		return nil
	})
}

// initNetwork creates the network.Network implementation with the given metrics, middleware, initial list of network
// participants and topology used to choose peers from the list of participants. The list of participants can later be
// updated by calling network.SetIDs.
func (builder *FlowAccessNodeBuilder) initNetwork(nodeID module.Local,
	networkMetrics module.NetworkMetrics,
	middleware network.Middleware,
	topology network.Topology,
	receiveCache *netcache.ReceiveCache,
) (*p2p.Network, error) {

	// creates network instance
	net, err := p2p.NewNetwork(&p2p.NetworkParameters{
		Logger:              builder.Logger,
		Codec:               cborcodec.NewCodec(),
		Me:                  nodeID,
		MiddlewareFactory:   func() (network.Middleware, error) { return builder.Middleware, nil },
		Topology:            topology,
		SubscriptionManager: subscription.NewChannelSubscriptionManager(middleware),
		Metrics:             networkMetrics,
		IdentityProvider:    builder.IdentityProvider,
		ReceiveCache:        receiveCache,
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize network: %w", err)
	}

	return net, nil
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

		// The following wrapper allows to black-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of blocked nodes to true
		builder.IdentityProvider, err = cache.NewNodeBlocklistWrapper(idCache, node.DB)
		if err != nil {
			return fmt.Errorf("could not initialize NodeBlocklistWrapper: %w", err)
		}

		builder.SyncEngineParticipantsProviderFactory = func() module.IdentifierProvider {
			return id.NewIdentityFilterIdentifierProvider(
				filter.And(
					filter.HasRole(flow.RoleConsensus),
					filter.Not(filter.HasNodeID(node.Me.NodeID())),
					p2p.NotEjectedFilter,
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
	return nil
}

func (builder *FlowAccessNodeBuilder) enqueueRelayNetwork() {
	builder.Component("relay network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		relayNet := relaynet.NewRelayNetwork(
			node.Network,
			builder.AccessNodeConfig.PublicNetworkConfig.Network,
			node.Logger,
			map[channels.Channel]channels.Channel{
				channels.ReceiveBlocks: channels.PublicReceiveBlocks,
			},
		)
		node.Network = relayNet
		return relayNet, nil
	})
}

func (builder *FlowAccessNodeBuilder) Build() (cmd.Node, error) {
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
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure(), //nolint:staticcheck
				backend.WithClientUnaryInterceptor(builder.rpcConf.CollectionClientTimeout))
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
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
					grpc.WithInsecure()) //nolint:staticcheck
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
			builder.TransactionMetrics = metrics.NewTransactionCollector(builder.TransactionTimings, node.Logger, builder.logTxTimeToFinalized,
				builder.logTxTimeToExecuted, builder.logTxTimeToFinalizedExecuted)
			return nil
		}).
		Module("access metrics", func(node *cmd.NodeConfig) error {
			builder.AccessMetrics = metrics.NewAccessCollector()
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
		Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			engineBuilder, err := rpc.NewBuilder(
				node.Logger,
				node.State,
				builder.rpcConf,
				builder.CollectionRPC,
				builder.HistoricalAccessRPCs,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Receipts,
				node.Storage.Results,
				node.RootChainID,
				builder.TransactionMetrics,
				builder.AccessMetrics,
				builder.collectionGRPCPort,
				builder.executionGRPCPort,
				builder.retryEnabled,
				builder.rpcMetricsEnabled,
				builder.apiRatelimits,
				builder.apiBurstlimits,
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

			return builder.RpcEng, nil
		}).
		Component("ingestion engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error

			builder.RequestEng, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
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
				node.Network,
				node.State,
				node.Me,
				builder.RequestEng,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				node.Storage.Transactions,
				node.Storage.Results,
				node.Storage.Receipts,
				builder.TransactionMetrics,
				builder.CollectionsToMarkFinalized,
				builder.CollectionsToMarkExecuted,
				builder.BlocksToMarkExecuted,
				builder.RpcEng,
			)
			if err != nil {
				return nil, err
			}
			builder.RequestEng.WithHandle(builder.IngestEng.OnCollection)
			builder.FinalizationDistributor.AddConsumer(builder.IngestEng)

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
				node.Storage.Blocks,
				builder.SyncCore,
				builder.FinalizedHeader,
			)

			if err != nil {
				return nil, fmt.Errorf("could not create public sync request handler: %w", err)
			}

			return syncRequestHandler, nil
		})
	}

	if builder.executionDataSyncEnabled {
		builder.BuildExecutionDataRequester()
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
	var libp2pNode *p2pnode.Node
	builder.
		Component("public libp2p node", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			libP2PFactory := builder.initLibP2PFactory(builder.NodeConfig.NetworkKey)

			var err error
			libp2pNode, err = libP2PFactory()
			if err != nil {
				return nil, fmt.Errorf("could not create public libp2p node: %w", err)
			}

			return libp2pNode, nil
		}).
		Component("public network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			builder.PublicNetworkConfig.Metrics = metrics.NewNetworkCollector(metrics.WithNetworkPrefix("public"))

			msgValidators := publicNetworkMsgValidators(node.Logger.With().Bool("public", true).Logger(), node.IdentityProvider, builder.NodeID)

			middleware := builder.initMiddleware(builder.NodeID, builder.PublicNetworkConfig.Metrics, libp2pNode, msgValidators...)

			// topology returns empty list since peers are not known upfront
			top := topology.EmptyTopology{}

			var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
			if builder.HeroCacheMetricsEnable {
				heroCacheCollector = metrics.PublicNetworkReceiveCacheMetricsFactory(builder.MetricsRegisterer)
			}
			receiveCache := netcache.NewHeroReceiveCache(builder.NetworkReceivedMessageCacheSize,
				builder.Logger,
				heroCacheCollector)

			err := node.Metrics.Mempool.Register(metrics.ResourcePublicNetworkingReceiveCache, receiveCache.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register networking receive cache metric: %w", err)
			}

			net, err := builder.initNetwork(builder.Me, builder.PublicNetworkConfig.Metrics, middleware, top, receiveCache)
			if err != nil {
				return nil, err
			}

			builder.AccessNodeConfig.PublicNetworkConfig.Network = net

			node.Logger.Info().Msgf("network will run on address: %s", builder.PublicNetworkConfig.BindAddress)
			return net, nil
		}).
		Component("public peer manager", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			return libp2pNode.PeerManagerComponent(), nil
		})
}

// initLibP2PFactory creates the LibP2P factory function for the given node ID and network key.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
// The LibP2P host is created with the following options:
//   - DHT as server
//   - The address from the node config or the specified bind address as the listen address
//   - The passed in private key as the libp2p key
//   - No connection gater
//   - Default Flow libp2p pubsub options
func (builder *FlowAccessNodeBuilder) initLibP2PFactory(networkKey crypto.PrivateKey) p2pbuilder.LibP2PFactoryFunc {
	return func() (*p2pnode.Node, error) {
		connManager := connection.NewConnManager(builder.Logger, builder.PublicNetworkConfig.Metrics)

		libp2pNode, err := p2pbuilder.NewNodeBuilder(builder.Logger, builder.PublicNetworkConfig.BindAddress, networkKey, builder.SporkID).
			SetBasicResolver(builder.Resolver).
			SetSubscriptionFilter(
				subscription.NewRoleBasedFilter(
					flow.RoleAccess, builder.IdentityProvider,
				),
			).
			SetConnectionManager(connManager).
			SetRoutingSystem(func(ctx context.Context, h host.Host) (routing.Routing, error) {
				return dht.NewDHT(
					ctx,
					h,
					unicast.FlowPublicDHTProtocolID(builder.SporkID),
					builder.Logger,
					builder.PublicNetworkConfig.Metrics,
					dht.AsServer(),
				)
			}).
			// disable connection pruning for the access node which supports the observer
			SetPeerManagerOptions(connection.ConnectionPruningDisabled, builder.PeerUpdateInterval).
			Build()

		if err != nil {
			return nil, fmt.Errorf("could not build libp2p node for staked access node: %w", err)
		}

		builder.LibP2PNode = libp2pNode

		return builder.LibP2PNode, nil
	}
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (builder *FlowAccessNodeBuilder) initMiddleware(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	libp2pNode *p2pnode.Node,
	validators ...network.MessageValidator) network.Middleware {

	logger := builder.Logger.With().Bool("staked", false).Logger()

	slashingViolationsConsumer := slashing.NewSlashingViolationsConsumer(logger, builder.Metrics.Network)

	builder.Middleware = middleware.NewMiddleware(
		logger,
		libp2pNode,
		nodeID,
		networkMetrics,
		builder.Metrics.Bitswap,
		builder.SporkID,
		middleware.DefaultUnicastTimeout,
		builder.IDTranslator,
		builder.CodecFactory(),
		slashingViolationsConsumer,
		middleware.WithMessageValidators(validators...),
		// use default identifier provider
	)

	return builder.Middleware
}
