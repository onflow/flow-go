package node_builder

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/admin/commands"
	stateSyncCommands "github.com/onflow/flow-go/admin/commands/state_synchronization"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/apiproxy"
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/ingestion/collections"
	"github.com/onflow/flow-go/engine/access/rest"
	restapiproxy "github.com/onflow/flow-go/engine/access/rest/apiproxy"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/rpc/backend/events"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	rpcConnection "github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/access/state_stream"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	subscriptiontracker "github.com/onflow/flow-go/engine/access/subscription/tracker"
	"github.com/onflow/flow-go/engine/common/follower"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/stop"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	execdatacache "github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/pruner"
	edstorage "github.com/onflow/flow-go/module/executiondatasync/storage"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	edrequester "github.com/onflow/flow-go/module/state_synchronization/requester"
	consensus_follower "github.com/onflow/flow-go/module/upstream"
	"github.com/onflow/flow-go/network"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/converter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/underlay"
	"github.com/onflow/flow-go/network/validator"
	stateprotocol "github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	pstorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/io"
)

// ObserverBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Access node
// These functions are shared by observer builders.
// The Staked network allows the access nodes to communicate among themselves, while the public network allows the
// observers and an Access node to communicate.
//
//                                 public network                           private network
//  +------------------------+
//  | observer 1             |<--------------------------|
//  +------------------------+                           v
//  +------------------------+                         +----------------------+              +------------------------+
//  | observer 2             |<----------------------->| Access Node (staked) |<------------>| All other staked Nodes |
//  +------------------------+                         +----------------------+              +------------------------+
//  +------------------------+                           ^
//  | observer 3             |<--------------------------|
//  +------------------------+

// ObserverServiceConfig defines all the user defined parameters required to bootstrap an access node
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type ObserverServiceConfig struct {
	observerNetworkingKeyPath            string
	bootstrapIdentities                  flow.IdentitySkeletonList // the identity list of bootstrap peers the node uses to discover other nodes
	apiRatelimits                        map[string]int
	apiBurstlimits                       map[string]int
	rpcConf                              rpc.Config
	rpcMetricsEnabled                    bool
	registersDBPath                      string
	checkpointFile                       string
	stateStreamConf                      statestreambackend.Config
	stateStreamFilterConf                map[string]int
	upstreamNodeAddresses                []string
	upstreamNodePublicKeys               []string
	upstreamIdentities                   flow.IdentitySkeletonList // the identity list of upstream peers the node uses to forward API requests to
	scriptExecutorConfig                 query.QueryConfig
	logTxTimeToFinalized                 bool
	logTxTimeToExecuted                  bool
	logTxTimeToFinalizedExecuted         bool
	logTxTimeToSealed                    bool
	executionDataSyncEnabled             bool
	executionDataIndexingEnabled         bool
	executionDataPrunerHeightRangeTarget uint64
	executionDataPrunerThreshold         uint64
	executionDataPruningInterval         time.Duration
	localServiceAPIEnabled               bool
	versionControlEnabled                bool
	stopControlEnabled                   bool
	executionDataDir                     string
	executionDataStartHeight             uint64
	executionDataConfig                  edrequester.ExecutionDataConfig
	scriptExecMinBlock                   uint64
	scriptExecMaxBlock                   uint64
	registerCacheType                    string
	registerCacheSize                    uint
	programCacheSize                     uint
	registerDBPruneThreshold             uint64
}

// DefaultObserverServiceConfig defines all the default values for the ObserverServiceConfig
func DefaultObserverServiceConfig() *ObserverServiceConfig {
	homedir, _ := os.UserHomeDir()
	return &ObserverServiceConfig{
		rpcConf: rpc.Config{
			UnsecureGRPCListenAddr: "0.0.0.0:9000",
			SecureGRPCListenAddr:   "0.0.0.0:9001",
			HTTPListenAddr:         "0.0.0.0:8000",
			CollectionAddr:         "",
			HistoricalAccessAddrs:  "",
			BackendConfig: backend.Config{
				AccessConfig:              rpcConnection.DefaultAccessConfig(),
				CollectionConfig:          rpcConnection.DefaultCollectionConfig(), // unused on observers
				ExecutionConfig:           rpcConnection.DefaultExecutionConfig(),  // unused on observers
				ConnectionPoolSize:        backend.DefaultConnectionPoolSize,
				MaxHeightRange:            events.DefaultMaxHeightRange,
				PreferredExecutionNodeIDs: nil,
				FixedExecutionNodeIDs:     nil,
				ScriptExecutionMode:       query_mode.IndexQueryModeExecutionNodesOnly.String(), // default to ENs only for now
				EventQueryMode:            query_mode.IndexQueryModeExecutionNodesOnly.String(), // default to ENs only for now
				TxResultQueryMode:         query_mode.IndexQueryModeExecutionNodesOnly.String(), // default to ENs only for now
			},
			RestConfig: rest.Config{
				ListenAddress:   "",
				WriteTimeout:    rest.DefaultWriteTimeout,
				ReadTimeout:     rest.DefaultReadTimeout,
				IdleTimeout:     rest.DefaultIdleTimeout,
				MaxRequestSize:  commonrpc.DefaultAccessMaxRequestSize,
				MaxResponseSize: commonrpc.DefaultAccessMaxResponseSize,
			},
			DeprecatedMaxMsgSize:      0,
			CompressorName:            grpcutils.NoCompressor,
			WebSocketConfig:           websockets.NewDefaultWebsocketConfig(),
			EnableWebSocketsStreamAPI: true,
		},
		stateStreamConf: statestreambackend.Config{
			MaxExecutionDataMsgSize: commonrpc.DefaultAccessMaxResponseSize,
			ExecutionDataCacheSize:  subscription.DefaultCacheSize,
			ClientSendTimeout:       subscription.DefaultSendTimeout,
			ClientSendBufferSize:    subscription.DefaultSendBufferSize,
			MaxGlobalStreams:        subscription.DefaultMaxGlobalStreams,
			EventFilterConfig:       state_stream.DefaultEventFilterConfig,
			ResponseLimit:           subscription.DefaultResponseLimit,
			HeartbeatInterval:       subscription.DefaultHeartbeatInterval,
			RegisterIDsRequestLimit: state_stream.DefaultRegisterIDsRequestLimit,
		},
		stateStreamFilterConf:                nil,
		rpcMetricsEnabled:                    false,
		apiRatelimits:                        nil,
		apiBurstlimits:                       nil,
		observerNetworkingKeyPath:            cmd.NotSet,
		upstreamNodeAddresses:                []string{},
		upstreamNodePublicKeys:               []string{},
		registersDBPath:                      filepath.Join(homedir, ".flow", "execution_state"),
		checkpointFile:                       cmd.NotSet,
		scriptExecutorConfig:                 query.NewDefaultConfig(),
		logTxTimeToFinalized:                 false,
		logTxTimeToExecuted:                  false,
		logTxTimeToFinalizedExecuted:         false,
		logTxTimeToSealed:                    false,
		executionDataSyncEnabled:             false,
		executionDataIndexingEnabled:         false,
		executionDataPrunerHeightRangeTarget: 0,
		executionDataPrunerThreshold:         pruner.DefaultThreshold,
		executionDataPruningInterval:         pruner.DefaultPruningInterval,
		localServiceAPIEnabled:               false,
		versionControlEnabled:                true,
		stopControlEnabled:                   false,
		executionDataDir:                     filepath.Join(homedir, ".flow", "execution_data"),
		executionDataStartHeight:             0,
		executionDataConfig: edrequester.ExecutionDataConfig{
			InitialBlockHeight: 0,
			MaxSearchAhead:     edrequester.DefaultMaxSearchAhead,
			FetchTimeout:       edrequester.DefaultFetchTimeout,
			MaxFetchTimeout:    edrequester.DefaultMaxFetchTimeout,
			RetryDelay:         edrequester.DefaultRetryDelay,
			MaxRetryDelay:      edrequester.DefaultMaxRetryDelay,
		},
		scriptExecMinBlock:       0,
		scriptExecMaxBlock:       math.MaxUint64,
		registerCacheType:        pstorage.CacheTypeTwoQueue.String(),
		registerCacheSize:        0,
		programCacheSize:         0,
		registerDBPruneThreshold: pruner.DefaultThreshold,
	}
}

// ObserverServiceBuilder provides the common functionality needed to bootstrap a Flow observer service
// It is composed of the FlowNodeBuilder, the ObserverServiceConfig and contains all the components and modules needed for the observers
type ObserverServiceBuilder struct {
	*cmd.FlowNodeBuilder
	*ObserverServiceConfig

	// components

	LibP2PNode           p2p.LibP2PNode
	FollowerState        stateprotocol.FollowerState
	SyncCore             *chainsync.Core
	RpcEng               *rpc.Engine
	TransactionTimings   *stdmap.TransactionTimings
	FollowerDistributor  *pubsub.FollowerDistributor
	Committee            hotstuff.DynamicCommittee
	Finalized            *flow.Header
	Pending              []*flow.ProposalHeader
	FollowerCore         module.HotStuffFollower
	ExecutionIndexer     *indexer.Indexer
	ExecutionIndexerCore *indexer.IndexerCore
	TxResultsIndex       *index.TransactionResultsIndex
	IndexerDependencies  *cmd.DependencyList
	VersionControl       *version.VersionControl
	StopControl          *stop.StopControl

	ExecutionDataDownloader   execution_data.Downloader
	ExecutionDataRequester    state_synchronization.ExecutionDataRequester
	ExecutionDataStore        execution_data.ExecutionDataStore
	ExecutionDataBlobstore    blobs.Blobstore
	ExecutionDataPruner       *pruner.Pruner
	ExecutionDatastoreManager edstorage.DatastoreManager
	ExecutionDataTracker      tracker.Storage

	RegistersAsyncStore *execution.RegistersAsyncStore
	Reporter            *index.Reporter
	EventsIndex         *index.EventsIndex
	ScriptExecutor      *backend.ScriptExecutor

	// storage
	events                  storage.Events
	lightTransactionResults storage.LightTransactionResults
	scheduledTransactions   storage.ScheduledTransactions

	// available until after the network has started. Hence, a factory function that needs to be called just before
	// creating the sync engine
	SyncEngineParticipantsProviderFactory func() module.IdentifierProvider

	// engines
	FollowerEng    *follower.ComplianceEngine
	SyncEng        *synceng.Engine
	StateStreamEng *statestreambackend.Engine

	// Public network
	peerID peer.ID

	TransactionMetrics *metrics.TransactionCollector
	RestMetrics        *metrics.RestCollector
	AccessMetrics      module.AccessMetrics

	// grpc servers
	secureGrpcServer      *grpcserver.GrpcServer
	unsecureGrpcServer    *grpcserver.GrpcServer
	stateStreamGrpcServer *grpcserver.GrpcServer

	stateStreamBackend *statestreambackend.StateStreamBackend
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstrap peers from the parameters.
// These are the identities of the observers also acting as the DHT bootstrap server
func (builder *ObserverServiceBuilder) deriveBootstrapPeerIdentities() error {
	// if bootstrap identities already provided (as part of alternate initialization as a library the skip reading command
	// line params)
	if builder.bootstrapIdentities != nil {
		return nil
	}

	ids, err := builder.DeriveBootstrapPeerIdentities()
	if err != nil {
		return fmt.Errorf("failed to derive bootstrap peer identities: %w", err)
	}

	builder.bootstrapIdentities = ids

	return nil
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstrap peers from the parameters.
// These are the identities of the observers also acting as the DHT bootstrap server
func (builder *ObserverServiceBuilder) deriveUpstreamIdentities() error {
	// if bootstrap identities already provided (as part of alternate initialization as a library the skip reading command
	// line params)
	if builder.upstreamIdentities != nil {
		return nil
	}

	// BootstrapIdentities converts the bootstrap node addresses and keys to a Flow Identity list where
	// each Flow Identity is initialized with the passed address, the networking key
	// and the Node ID set to ZeroID, role set to Access, 0 stake and no staking key.
	addresses := builder.upstreamNodeAddresses
	keys := builder.upstreamNodePublicKeys
	if len(addresses) != len(keys) {
		return fmt.Errorf("number of addresses and keys provided for the boostrap nodes don't match")
	}

	ids := make(flow.IdentitySkeletonList, len(addresses))
	for i, address := range addresses {
		key := keys[i]

		// json unmarshaller needs a quotes before and after the string
		// the pflags.StringSliceVar does not retain quotes for the command line arg even if escaped with \"
		// hence this additional check to ensure the key is indeed quoted
		if !strings.HasPrefix(key, "\"") {
			key = fmt.Sprintf("\"%s\"", key)
		}

		// create the identity of the peer by setting only the relevant fields
		ids[i] = &flow.IdentitySkeleton{
			NodeID:        flow.ZeroID, // the NodeID is the hash of the staking key and for the public network it does not apply
			Address:       address,
			Role:          flow.RoleAccess, // the upstream node has to be an access node
			NetworkPubKey: nil,
		}

		// networking public key
		var networkKey encodable.NetworkPubKey
		err := json.Unmarshal([]byte(key), &networkKey)
		if err == nil {
			ids[i].NetworkPubKey = networkKey
		}
	}

	builder.upstreamIdentities = ids

	return nil
}

func (builder *ObserverServiceBuilder) buildFollowerState() *ObserverServiceBuilder {
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

func (builder *ObserverServiceBuilder) buildSyncCore() *ObserverServiceBuilder {
	builder.Module("sync core", func(node *cmd.NodeConfig) error {
		syncCore, err := chainsync.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector(node.RootChainID), node.RootChainID)
		builder.SyncCore = syncCore

		return err
	})

	return builder
}

func (builder *ObserverServiceBuilder) buildCommittee() *ObserverServiceBuilder {
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

func (builder *ObserverServiceBuilder) buildLatestHeader() *ObserverServiceBuilder {
	builder.Module("latest header", func(node *cmd.NodeConfig) error {
		finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
		builder.Finalized, builder.Pending = finalized, pending

		return err
	})

	return builder
}

func (builder *ObserverServiceBuilder) buildFollowerCore() *ObserverServiceBuilder {
	builder.Component("follower core", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// create a finalizer that will handle updating the protocol
		// state when the follower detects newly finalized blocks
		final := finalizer.NewFinalizer(node.ProtocolDB.Reader(), node.Storage.Headers, builder.FollowerState, node.Tracer)

		followerCore, err := consensus.NewFollower(
			node.Logger,
			node.Metrics.Mempool,
			node.Storage.Headers,
			final,
			builder.FollowerDistributor,
			node.FinalizedRootBlock.ToHeader(),
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

func (builder *ObserverServiceBuilder) buildFollowerEngine() *ObserverServiceBuilder {
	builder.Component("follower engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
		if node.HeroCacheMetricsEnable {
			heroCacheCollector = metrics.FollowerCacheMetrics(node.MetricsRegisterer)
		}
		packer := hotsignature.NewConsensusSigDataPacker(builder.Committee)
		verifier := verification.NewCombinedVerifier(builder.Committee, packer) // verifier for HotStuff signature constructs (QCs, TCs, votes)
		val := hotstuffvalidator.New(builder.Committee, verifier)

		core, err := follower.NewComplianceCore(
			node.Logger,
			node.Metrics.Mempool,
			heroCacheCollector,
			builder.FollowerDistributor,
			builder.FollowerState,
			builder.FollowerCore,
			val,
			builder.SyncCore,
			node.Tracer,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower core: %w", err)
		}

		builder.FollowerEng, err = follower.NewComplianceLayer(
			node.Logger,
			node.EngineRegistry,
			node.Me,
			node.Metrics.Engine,
			node.Storage.Headers,
			builder.Finalized,
			core,
			builder.FollowerDistributor,
			builder.ComplianceConfig,
			follower.WithChannel(channels.PublicReceiveBlocks),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}

		return builder.FollowerEng, nil
	})

	return builder
}

func (builder *ObserverServiceBuilder) buildSyncEngine() *ObserverServiceBuilder {
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
			builder.FollowerDistributor,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create synchronization engine: %w", err)
		}
		builder.SyncEng = sync

		return builder.SyncEng, nil
	})

	return builder
}

func (builder *ObserverServiceBuilder) BuildConsensusFollower() cmd.NodeBuilder {
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

type Option func(*ObserverServiceConfig)

func NewFlowObserverServiceBuilder(opts ...Option) *ObserverServiceBuilder {
	config := DefaultObserverServiceConfig()
	for _, opt := range opts {
		opt(config)
	}
	anb := &ObserverServiceBuilder{
		ObserverServiceConfig: config,
		FlowNodeBuilder:       cmd.FlowNode("observer"),
		FollowerDistributor:   pubsub.NewFollowerDistributor(),
		IndexerDependencies:   cmd.NewDependencyList(),
	}
	anb.FollowerDistributor.AddProposalViolationConsumer(notifications.NewSlashingViolationsConsumer(anb.Logger))
	// the observer gets a version of the root snapshot file that does not contain any node addresses
	// hence skip all the root snapshot validations that involved an identity address
	anb.FlowNodeBuilder.SkipNwAddressBasedValidations = true
	return anb
}

func (builder *ObserverServiceBuilder) ParseFlags() error {
	builder.BaseFlags()

	builder.extraFlags()

	return builder.ParseAndPrintFlags()
}

func (builder *ObserverServiceBuilder) extraFlags() {
	builder.ExtraFlags(func(flags *pflag.FlagSet) {
		defaultConfig := DefaultObserverServiceConfig()

		flags.StringVarP(&builder.rpcConf.UnsecureGRPCListenAddr,
			"rpc-addr",
			"r",
			defaultConfig.rpcConf.UnsecureGRPCListenAddr,
			"the address the unsecured gRPC server listens on")
		flags.StringVar(&builder.rpcConf.SecureGRPCListenAddr,
			"secure-rpc-addr",
			defaultConfig.rpcConf.SecureGRPCListenAddr,
			"the address the secure gRPC server listens on")
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
		flags.Int64Var(&builder.rpcConf.RestConfig.MaxRequestSize,
			"rest-max-request-size",
			defaultConfig.rpcConf.RestConfig.MaxRequestSize,
			"the maximum request size in bytes for payload sent over REST server")
		flags.Int64Var(&builder.rpcConf.RestConfig.MaxResponseSize,
			"rest-max-response-size",
			defaultConfig.rpcConf.RestConfig.MaxResponseSize,
			"the maximum response size in bytes for payload sent from REST server")
		flags.UintVar(&builder.rpcConf.DeprecatedMaxMsgSize,
			"rpc-max-message-size",
			defaultConfig.rpcConf.DeprecatedMaxMsgSize,
			"[deprecated] the maximum message size in bytes for messages sent or received over grpc")
		flags.UintVar(&builder.rpcConf.BackendConfig.AccessConfig.MaxRequestMsgSize,
			"rpc-max-request-message-size",
			defaultConfig.rpcConf.BackendConfig.AccessConfig.MaxRequestMsgSize,
			"the maximum request message size in bytes for request messages received over grpc by the server")
		flags.UintVar(&builder.rpcConf.BackendConfig.AccessConfig.MaxResponseMsgSize,
			"rpc-max-response-message-size",
			defaultConfig.rpcConf.BackendConfig.AccessConfig.MaxResponseMsgSize,
			"the maximum message size in bytes for response messages sent over grpc by the server")
		flags.UintVar(&builder.rpcConf.BackendConfig.ConnectionPoolSize,
			"connection-pool-size",
			defaultConfig.rpcConf.BackendConfig.ConnectionPoolSize,
			"maximum number of connections allowed in the connection pool, size of 0 disables the connection pooling, and anything less than the default size will be overridden to use the default size")
		flags.UintVar(&builder.rpcConf.BackendConfig.MaxHeightRange,
			"rpc-max-height-range",
			defaultConfig.rpcConf.BackendConfig.MaxHeightRange,
			"maximum size for height range requests")
		flags.StringToIntVar(&builder.apiRatelimits,
			"api-rate-limits",
			defaultConfig.apiRatelimits,
			"per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&builder.apiBurstlimits,
			"api-burst-limits",
			defaultConfig.apiBurstlimits,
			"burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.StringVar(&builder.observerNetworkingKeyPath,
			"observer-networking-key-path",
			defaultConfig.observerNetworkingKeyPath,
			"path to the networking key for observer")
		flags.DurationVar(&builder.rpcConf.BackendConfig.AccessConfig.Timeout,
			"upstream-api-timeout",
			defaultConfig.rpcConf.BackendConfig.AccessConfig.Timeout,
			"tcp timeout for Flow API gRPC sockets to upstrem nodes")
		flags.StringSliceVar(&builder.upstreamNodeAddresses,
			"upstream-node-addresses",
			defaultConfig.upstreamNodeAddresses,
			"the gRPC network addresses of the upstream access node. e.g. access-001.mainnet.flow.org:9000,access-002.mainnet.flow.org:9000")
		flags.StringSliceVar(&builder.upstreamNodePublicKeys,
			"upstream-node-public-keys",
			defaultConfig.upstreamNodePublicKeys,
			"the networking public key of the upstream access node (in the same order as the upstream node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")

		flags.BoolVar(&builder.logTxTimeToFinalized, "log-tx-time-to-finalized", defaultConfig.logTxTimeToFinalized, "log transaction time to finalized")
		flags.BoolVar(&builder.logTxTimeToExecuted, "log-tx-time-to-executed", defaultConfig.logTxTimeToExecuted, "log transaction time to executed")
		flags.BoolVar(&builder.logTxTimeToFinalizedExecuted,
			"log-tx-time-to-finalized-executed",
			defaultConfig.logTxTimeToFinalizedExecuted,
			"log transaction time to finalized and executed")
		flags.BoolVar(&builder.logTxTimeToSealed,
			"log-tx-time-to-sealed",
			defaultConfig.logTxTimeToSealed,
			"log transaction time to sealed")
		flags.BoolVar(&builder.rpcMetricsEnabled, "rpc-metrics-enabled", defaultConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")
		flags.BoolVar(&builder.executionDataIndexingEnabled,
			"execution-data-indexing-enabled",
			defaultConfig.executionDataIndexingEnabled,
			"whether to enable the execution data indexing")
		flags.BoolVar(&builder.versionControlEnabled,
			"version-control-enabled",
			defaultConfig.versionControlEnabled,
			"whether to enable the version control feature. Default value is true")
		flags.BoolVar(&builder.stopControlEnabled,
			"stop-control-enabled",
			defaultConfig.stopControlEnabled,
			"whether to enable the stop control feature. Default value is false")
		flags.BoolVar(&builder.localServiceAPIEnabled, "local-service-api-enabled", defaultConfig.localServiceAPIEnabled, "whether to use local indexed data for api queries")
		flags.StringVar(&builder.registersDBPath, "execution-state-dir", defaultConfig.registersDBPath, "directory to use for execution-state database")
		flags.StringVar(&builder.checkpointFile, "execution-state-checkpoint", defaultConfig.checkpointFile, "execution-state checkpoint file")

		var builderExecutionDataDBMode string
		flags.StringVar(&builderExecutionDataDBMode, "execution-data-db", "pebble", "[deprecated] the DB type for execution datastore.")

		// Execution data pruner
		flags.Uint64Var(&builder.executionDataPrunerHeightRangeTarget,
			"execution-data-height-range-target",
			defaultConfig.executionDataPrunerHeightRangeTarget,
			"number of blocks of Execution Data to keep on disk. older data is pruned")
		flags.Uint64Var(&builder.executionDataPrunerThreshold,
			"execution-data-height-range-threshold",
			defaultConfig.executionDataPrunerThreshold,
			"number of unpruned blocks of Execution Data beyond the height range target to allow before pruning")
		flags.DurationVar(&builder.executionDataPruningInterval,
			"execution-data-pruning-interval",
			defaultConfig.executionDataPruningInterval,
			"duration after which the pruner tries to prune execution data. The default value is 10 minutes")

		// ExecutionDataRequester config
		flags.BoolVar(&builder.executionDataSyncEnabled,
			"execution-data-sync-enabled",
			defaultConfig.executionDataSyncEnabled,
			"whether to enable the execution data sync protocol")
		flags.StringVar(&builder.executionDataDir,
			"execution-data-dir",
			defaultConfig.executionDataDir,
			"directory to use for Execution Data database")
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

		// Streaming API
		flags.StringVar(&builder.stateStreamConf.ListenAddr,
			"state-stream-addr",
			defaultConfig.stateStreamConf.ListenAddr,
			"the address the state stream server listens on (if empty the server will not be started)")
		flags.Uint32Var(&builder.stateStreamConf.ExecutionDataCacheSize,
			"execution-data-cache-size",
			defaultConfig.stateStreamConf.ExecutionDataCacheSize,
			"block execution data cache size")
		flags.Uint32Var(&builder.stateStreamConf.MaxGlobalStreams,
			"state-stream-global-max-streams", defaultConfig.stateStreamConf.MaxGlobalStreams,
			"global maximum number of concurrent streams")
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
		flags.StringVar(&builder.rpcConf.BackendConfig.EventQueryMode,
			"event-query-mode",
			defaultConfig.rpcConf.BackendConfig.EventQueryMode,
			"mode to use when querying events. one of [local-only, execution-nodes-only(default), failover]")
		flags.Uint64Var(&builder.scriptExecMinBlock,
			"script-execution-min-height",
			defaultConfig.scriptExecMinBlock,
			"lowest block height to allow for script execution. default: no limit")
		flags.Uint64Var(&builder.scriptExecMaxBlock,
			"script-execution-max-height",
			defaultConfig.scriptExecMaxBlock,
			"highest block height to allow for script execution. default: no limit")

		flags.StringVar(&builder.registerCacheType,
			"register-cache-type",
			defaultConfig.registerCacheType,
			"type of backend cache to use for registers (lru, arc, 2q)")
		flags.UintVar(&builder.registerCacheSize,
			"register-cache-size",
			defaultConfig.registerCacheSize,
			"number of registers to cache for script execution. default: 0 (no cache)")
		flags.UintVar(&builder.programCacheSize,
			"program-cache-size",
			defaultConfig.programCacheSize,
			"[experimental] number of blocks to cache for cadence programs. use 0 to disable cache. default: 0. Note: this is an experimental feature and may cause nodes to become unstable under certain workloads. Use with caution.")

		// Register DB Pruning
		flags.Uint64Var(&builder.registerDBPruneThreshold,
			"registerdb-pruning-threshold",
			defaultConfig.registerDBPruneThreshold,
			fmt.Sprintf("specifies the number of blocks below the latest stored block height to keep in register db. default: %d", defaultConfig.registerDBPruneThreshold))

		// websockets config
		flags.DurationVar(
			&builder.rpcConf.WebSocketConfig.InactivityTimeout,
			"websocket-inactivity-timeout",
			defaultConfig.rpcConf.WebSocketConfig.InactivityTimeout,
			"the duration a WebSocket connection can remain open without any active subscriptions before being automatically closed",
		)
		flags.Uint64Var(
			&builder.rpcConf.WebSocketConfig.MaxSubscriptionsPerConnection,
			"websocket-max-subscriptions-per-connection",
			defaultConfig.rpcConf.WebSocketConfig.MaxSubscriptionsPerConnection,
			"the maximum number of active WebSocket subscriptions allowed per connection",
		)
		flags.Float64Var(
			&builder.rpcConf.WebSocketConfig.MaxResponsesPerSecond,
			"websocket-max-responses-per-second",
			defaultConfig.rpcConf.WebSocketConfig.MaxResponsesPerSecond,
			fmt.Sprintf("the maximum number of responses that can be sent to a single client per second. Default: %f. if set to 0, no limit is applied to the number of responses per second.", defaultConfig.rpcConf.WebSocketConfig.MaxResponsesPerSecond),
		)
		flags.BoolVar(
			&builder.rpcConf.EnableWebSocketsStreamAPI,
			"websockets-stream-api-enabled",
			defaultConfig.rpcConf.EnableWebSocketsStreamAPI,
			"whether to enable the WebSockets Stream API.",
		)
	}).ValidateFlags(func() error {
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
			if len(builder.stateStreamFilterConf) > 4 {
				return errors.New("state-stream-event-filter-limits must have at most 4 keys (EventTypes, Addresses, Contracts, AccountAddresses)")
			}
			for key, value := range builder.stateStreamFilterConf {
				switch key {
				case "EventTypes", "Addresses", "Contracts", "AccountAddresses":
					if value <= 0 {
						return fmt.Errorf("state-stream-event-filter-limits %s must be greater than 0", key)
					}
				default:
					return errors.New("state-stream-event-filter-limits may only contain the keys EventTypes, Addresses, Contracts, AccountAddresses")
				}
			}
			if builder.stateStreamConf.ResponseLimit < 0 {
				return errors.New("state-stream-response-limit must be greater than or equal to 0")
			}
			if builder.stateStreamConf.RegisterIDsRequestLimit <= 0 {
				return errors.New("state-stream-max-register-values must be greater than 0")
			}
		}

		if builder.rpcConf.RestConfig.MaxRequestSize <= 0 {
			return errors.New("rest-max-request-size must be greater than 0")
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

func (builder *ObserverServiceBuilder) initNodeInfo() error {
	// use the networking key that was loaded from the configured file
	networkingKey, err := loadNetworkingKey(builder.observerNetworkingKeyPath)
	if err != nil {
		return fmt.Errorf("could not load networking private key: %w", err)
	}

	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(networkingKey.PublicKey())
	if err != nil {
		return fmt.Errorf("could not load networking public key: %w", err)
	}

	builder.peerID, err = peer.IDFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("could not get peer ID from public key: %w", err)
	}

	builder.NodeID, err = translator.NewPublicNetworkIDTranslator().GetFlowID(builder.peerID)
	if err != nil {
		return fmt.Errorf("could not get flow node ID: %w", err)
	}

	builder.NodeConfig.NetworkKey = networkingKey // copy the key to NodeConfig
	builder.NodeConfig.StakingKey = nil           // no staking key for the observer

	return nil
}

func (builder *ObserverServiceBuilder) InitIDProviders() {
	builder.Module("id providers", func(node *cmd.NodeConfig) error {
		idCache, err := cache.NewProtocolStateIDCache(node.Logger, node.State, builder.ProtocolEvents)
		if err != nil {
			return fmt.Errorf("could not initialize ProtocolStateIDCache: %w", err)
		}
		builder.IDTranslator = translator.NewHierarchicalIDTranslator(idCache, translator.NewPublicNetworkIDTranslator())

		// The following wrapper allows to black-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of disallow-listed nodes to true
		builder.IdentityProvider, err = cache.NewNodeDisallowListWrapper(
			idCache,
			node.ProtocolDB,
			func() network.DisallowListNotificationConsumer {
				return builder.NetworkUnderlay
			},
		)
		if err != nil {
			return fmt.Errorf("could not initialize NodeDisallowListWrapper: %w", err)
		}

		// use the default identifier provider
		builder.SyncEngineParticipantsProviderFactory = func() module.IdentifierProvider {
			return id.NewCustomIdentifierProvider(func() flow.IdentifierList {
				pids := builder.LibP2PNode.GetPeersForProtocol(protocols.FlowProtocolID(builder.SporkID))
				result := make(flow.IdentifierList, 0, len(pids))

				for _, pid := range pids {
					// exclude own Identifier
					if pid == builder.peerID {
						continue
					}

					if flowID, err := builder.IDTranslator.GetFlowID(pid); err != nil {
						// TODO: this is an instance of "log error and continue with best effort" anti-pattern
						builder.Logger.Debug().Str("peer", p2plogging.PeerId(pid)).Msg("failed to translate to Flow ID")
					} else {
						result = append(result, flowID)
					}
				}

				return result
			})
		}

		return nil
	})
}

func (builder *ObserverServiceBuilder) Initialize() error {
	if err := builder.deriveBootstrapPeerIdentities(); err != nil {
		return err
	}

	if err := builder.deriveUpstreamIdentities(); err != nil {
		return err
	}

	if err := builder.validateParams(); err != nil {
		return err
	}

	if err := builder.initNodeInfo(); err != nil {
		return err
	}

	builder.InitIDProviders()

	builder.enqueuePublicNetworkInit()

	builder.enqueueConnectWithStakedAN()

	if builder.BaseConfig.MetricsEnabled {
		builder.EnqueueMetricsServerInit()
		if err := builder.RegisterBadgerMetrics(); err != nil {
			return err
		}
	}

	builder.PreInit(builder.initObserverLocal())

	return nil
}

func (builder *ObserverServiceBuilder) validateParams() error {
	if builder.BaseConfig.BindAddr == cmd.NotSet || builder.BaseConfig.BindAddr == "" {
		return errors.New("bind address not specified")
	}
	if builder.ObserverServiceConfig.observerNetworkingKeyPath == cmd.NotSet {
		return errors.New("networking key not provided")
	}
	if len(builder.bootstrapIdentities) > 0 {
		return nil
	}
	if len(builder.BootstrapNodeAddresses) == 0 {
		return errors.New("no bootstrap node address provided")
	}
	if len(builder.BootstrapNodeAddresses) != len(builder.BootstrapNodePublicKeys) {
		return errors.New("number of bootstrap node addresses and public keys should match")
	}
	if len(builder.upstreamNodePublicKeys) > 0 && len(builder.upstreamNodeAddresses) != len(builder.upstreamNodePublicKeys) {
		return errors.New("number of upstream node addresses and public keys must match if public keys given")
	}
	return nil
}

// initObserverLocal initializes the observer's ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (builder *ObserverServiceBuilder) initObserverLocal() func(node *cmd.NodeConfig) error {
	return func(node *cmd.NodeConfig) error {
		// for an observer, set the identity here explicitly since it will not be found in the protocol state
		self := flow.IdentitySkeleton{
			NodeID:        node.NodeID,
			NetworkPubKey: node.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the observer
			Role:          flow.RoleAccess, // observer can only run as an access node
			Address:       builder.BindAddr,
		}

		var err error
		node.Me, err = local.NewNoKey(self)
		if err != nil {
			return fmt.Errorf("could not initialize local: %w", err)
		}
		return nil
	}
}

// Build enqueues the sync engine and the follower engine for the observer.
// Currently, the observer only runs the follower engine.
func (builder *ObserverServiceBuilder) Build() (cmd.Node, error) {
	builder.BuildConsensusFollower()

	if builder.executionDataSyncEnabled {
		builder.BuildExecutionSyncComponents()
	}

	builder.enqueueRPCServer()
	return builder.FlowNodeBuilder.Build()
}

func (builder *ObserverServiceBuilder) BuildExecutionSyncComponents() *ObserverServiceBuilder {
	var ds datastore.Batching
	var bs network.BlobService
	var processedBlockHeightInitializer storage.ConsumerProgressInitializer
	var processedNotificationsInitializer storage.ConsumerProgressInitializer
	var publicBsDependable *module.ProxiedReadyDoneAware
	var execDataDistributor *edrequester.ExecutionDataDistributor
	var execDataCacheBackend *herocache.BlockExecutionData
	var executionDataStoreCache *execdatacache.ExecutionDataCache

	// setup dependency chain to ensure indexer starts after the requester
	requesterDependable := module.NewProxiedReadyDoneAware()
	builder.IndexerDependencies.Add(requesterDependable)

	executionDataPrunerEnabled := builder.executionDataPrunerHeightRangeTarget != 0

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

			builder.ExecutionDatastoreManager, err = edstorage.NewPebbleDatastoreManager(
				node.Logger.With().Str("pebbledb", "endata").Logger(),
				datastoreDir, nil)
			if err != nil {
				return fmt.Errorf("could not create PebbleDatastoreManager for execution data: %w", err)
			}
			ds = builder.ExecutionDatastoreManager.Datastore()

			builder.ShutdownFunc(func() error {
				if err := builder.ExecutionDatastoreManager.Close(); err != nil {
					return fmt.Errorf("could not close execution data datastore: %w", err)
				}
				return nil
			})

			return nil
		}).
		Module("processed block height consumer progress", func(node *cmd.NodeConfig) error {
			// Note: progress is stored in the datastore's DB since that is where the jobqueue
			// writes execution data to.
			db := builder.ExecutionDatastoreManager.DB()

			processedBlockHeightInitializer = store.NewConsumerProgress(db, module.ConsumeProgressExecutionDataRequesterBlockHeight)
			return nil
		}).
		Module("processed notifications consumer progress", func(node *cmd.NodeConfig) error {
			// Note: progress is stored in the datastore's DB since that is where the jobqueue
			// writes execution data to.
			db := builder.ExecutionDatastoreManager.DB()

			processedNotificationsInitializer = store.NewConsumerProgress(db, module.ConsumeProgressExecutionDataRequesterNotification)
			return nil
		}).
		Module("blobservice peer manager dependencies", func(node *cmd.NodeConfig) error {
			publicBsDependable = module.NewProxiedReadyDoneAware()
			builder.PeerManagerDependencies.Add(publicBsDependable)
			return nil
		}).
		Module("execution datastore", func(node *cmd.NodeConfig) error {
			builder.ExecutionDataBlobstore = blobs.NewBlobstore(ds)
			builder.ExecutionDataStore = execution_data.NewExecutionDataStore(builder.ExecutionDataBlobstore, execution_data.DefaultSerializer)
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
		Component("public execution data service", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			opts := []network.BlobServiceOption{
				blob.WithBitswapOptions(
					bitswap.WithTracer(
						blob.NewTracer(node.Logger.With().Str("public_blob_service", channels.PublicExecutionDataService.String()).Logger()),
					),
				),
			}

			var err error
			bs, err = node.EngineRegistry.RegisterBlobService(channels.PublicExecutionDataService, ds, opts...)
			if err != nil {
				return nil, fmt.Errorf("could not register blob service: %w", err)
			}

			// add blobservice into ReadyDoneAware dependency passed to peer manager
			// this starts the blob service and configures peer manager to wait for the blobservice
			// to be ready before starting
			publicBsDependable.Init(bs)

			var downloaderOpts []execution_data.DownloaderOption

			if executionDataPrunerEnabled {
				sealed, err := node.State.Sealed().Head()
				if err != nil {
					return nil, fmt.Errorf("cannot get the sealed block: %w", err)
				}

				trackerDir := filepath.Join(builder.executionDataDir, "tracker")
				builder.ExecutionDataTracker, err = tracker.OpenStorage(
					trackerDir,
					sealed.Height,
					node.Logger,
					tracker.WithPruneCallback(func(c cid.Cid) error {
						// TODO: use a proper context here
						return builder.ExecutionDataBlobstore.DeleteBlob(context.TODO(), c)
					}),
				)
				if err != nil {
					return nil, fmt.Errorf("failed to create execution data tracker: %w", err)
				}

				downloaderOpts = []execution_data.DownloaderOption{
					execution_data.WithExecutionDataTracker(builder.ExecutionDataTracker, node.Storage.Headers),
				}
			}

			builder.ExecutionDataDownloader = execution_data.NewDownloader(bs, downloaderOpts...)

			return builder.ExecutionDataDownloader, nil
		}).
		Component("execution data requester", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// Validation of the start block height needs to be done after loading state
			if builder.executionDataStartHeight > 0 {
				if builder.executionDataStartHeight <= builder.FinalizedRootBlock.Height {
					return nil, fmt.Errorf(
						"execution data start block height (%d) must be greater than the root block height (%d)",
						builder.executionDataStartHeight, builder.FinalizedRootBlock.Height)
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
				builder.executionDataConfig.InitialBlockHeight = builder.SealedRootBlock.Height
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

			// start processing from the initial block height resolved from the execution data config
			processedBlockHeight, err := processedBlockHeightInitializer.Initialize(builder.executionDataConfig.InitialBlockHeight)
			if err != nil {
				return nil, fmt.Errorf("could not initialize processed block height: %w", err)
			}
			processedNotifications, err := processedNotificationsInitializer.Initialize(builder.executionDataConfig.InitialBlockHeight)
			if err != nil {
				return nil, fmt.Errorf("could not initialize processed notifications: %w", err)
			}

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
				builder.FollowerDistributor,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create execution data requester: %w", err)
			}
			builder.ExecutionDataRequester = r

			// add requester into ReadyDoneAware dependency passed to indexer. This allows the indexer
			// to wait for the requester to be ready before starting.
			requesterDependable.Init(builder.ExecutionDataRequester)

			return builder.ExecutionDataRequester, nil
		}).
		Component("execution data pruner", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			if !executionDataPrunerEnabled {
				return &module.NoopReadyDoneAware{}, nil
			}

			var prunerMetrics module.ExecutionDataPrunerMetrics = metrics.NewNoopCollector()
			if node.MetricsEnabled {
				prunerMetrics = metrics.NewExecutionDataPrunerCollector()
			}

			var err error
			builder.ExecutionDataPruner, err = pruner.NewPruner(
				node.Logger,
				prunerMetrics,
				builder.ExecutionDataTracker,
				pruner.WithPruneCallback(func(ctx context.Context) error {
					return builder.ExecutionDatastoreManager.CollectGarbage(ctx)
				}),
				pruner.WithHeightRangeTarget(builder.executionDataPrunerHeightRangeTarget),
				pruner.WithThreshold(builder.executionDataPrunerThreshold),
				pruner.WithPruningInterval(builder.executionDataPruningInterval),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create execution data pruner: %w", err)
			}

			builder.ExecutionDataPruner.RegisterHeightRecorder(builder.ExecutionDataDownloader)

			return builder.ExecutionDataPruner, nil
		})
	if builder.executionDataIndexingEnabled {
		var indexedBlockHeightInitializer storage.ConsumerProgressInitializer

		builder.Module("indexed block height consumer progress", func(node *cmd.NodeConfig) error {
			// Note: progress is stored in the MAIN db since that is where indexed execution data is stored.
			indexedBlockHeightInitializer = store.NewConsumerProgress(builder.ProtocolDB, module.ConsumeProgressExecutionDataIndexerBlockHeight)
			return nil
		}).Module("transaction results storage", func(node *cmd.NodeConfig) error {
			builder.lightTransactionResults = store.NewLightTransactionResults(node.Metrics.Cache, node.ProtocolDB, bstorage.DefaultCacheSize)
			return nil
		}).Module("scheduled transactions storage", func(node *cmd.NodeConfig) error {
			builder.scheduledTransactions = store.NewScheduledTransactions(node.Metrics.Cache, node.ProtocolDB, bstorage.DefaultCacheSize)
			return nil
		}).DependableComponent("execution data indexer", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// Note: using a DependableComponent here to ensure that the indexer does not block
			// other components from starting while bootstrapping the register db since it may
			// take hours to complete.

			pdb, err := pstorage.OpenRegisterPebbleDB(
				node.Logger.With().Str("pebbledb", "registers").Logger(),
				builder.registersDBPath)
			if err != nil {
				return nil, fmt.Errorf("could not open registers db: %w", err)
			}
			builder.ShutdownFunc(func() error {
				return pdb.Close()
			})

			bootstrapped, err := pstorage.IsBootstrapped(pdb)
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

				checkpointHeight := builder.SealedRootBlock.Height

				if builder.SealedRootBlock.ID() != builder.RootSeal.BlockID {
					return nil, fmt.Errorf("mismatching sealed root block and root seal: %v != %v",
						builder.SealedRootBlock.ID(), builder.RootSeal.BlockID)
				}

				rootHash := ledger.RootHash(builder.RootSeal.FinalState)
				bootstrap, err := pstorage.NewRegisterBootstrap(pdb, checkpointFile, checkpointHeight, rootHash, builder.Logger)
				if err != nil {
					return nil, fmt.Errorf("could not create registers bootstrap: %w", err)
				}

				// TODO: find a way to hook a context up to this to allow a graceful shutdown
				workerCount := 10
				err = bootstrap.IndexCheckpointFile(context.Background(), workerCount)
				if err != nil {
					return nil, fmt.Errorf("could not load checkpoint file: %w", err)
				}
			}

			registers, err := pstorage.NewRegisters(pdb, builder.registerDBPruneThreshold)
			if err != nil {
				return nil, fmt.Errorf("could not create registers storage: %w", err)
			}

			if builder.registerCacheSize > 0 {
				cacheType, err := pstorage.ParseCacheType(builder.registerCacheType)
				if err != nil {
					return nil, fmt.Errorf("could not parse register cache type: %w", err)
				}
				cacheMetrics := metrics.NewCacheCollector(builder.RootChainID)
				registersCache, err := pstorage.NewRegistersCache(registers, cacheType, builder.registerCacheSize, cacheMetrics)
				if err != nil {
					return nil, fmt.Errorf("could not create registers cache: %w", err)
				}
				builder.Storage.RegisterIndex = registersCache
			} else {
				builder.Storage.RegisterIndex = registers
			}

			indexerDerivedChainData, queryDerivedChainData, err := builder.buildDerivedChainData()
			if err != nil {
				return nil, fmt.Errorf("could not create derived chain data: %w", err)
			}

			rootBlockHeight := node.State.Params().FinalizedRoot().Height
			progress, err := store.NewConsumerProgress(builder.ProtocolDB, module.ConsumeProgressLastFullBlockHeight).Initialize(rootBlockHeight)
			if err != nil {
				return nil, fmt.Errorf("could not create last full block height consumer progress: %w", err)
			}

			lastFullBlockHeight, err := counters.NewPersistentStrictMonotonicCounter(progress)
			if err != nil {
				return nil, fmt.Errorf("could not create last full block height counter: %w", err)
			}

			var collectionExecutedMetric module.CollectionExecutedMetric = metrics.NewNoopCollector()
			collectionIndexer, err := collections.NewIndexer(
				builder.Logger,
				builder.ProtocolDB,
				collectionExecutedMetric,
				builder.State,
				builder.Storage.Blocks,
				builder.Storage.Collections,
				lastFullBlockHeight,
				builder.StorageLockMgr,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create collection indexer: %w", err)
			}

			builder.ExecutionIndexerCore = indexer.New(
				builder.Logger,
				metrics.NewExecutionStateIndexerCollector(),
				builder.ProtocolDB,
				builder.Storage.RegisterIndex,
				builder.Storage.Headers,
				builder.events,
				builder.Storage.Collections,
				builder.Storage.Transactions,
				builder.lightTransactionResults,
				builder.scheduledTransactions,
				builder.RootChainID,
				indexerDerivedChainData,
				collectionIndexer,
				collectionExecutedMetric,
				node.StorageLockMgr,
			)

			// start processing from the first height of the registers db, which is initialized from
			// the checkpoint. this ensures a consistent starting point for the indexed data.
			indexedBlockHeight, err := indexedBlockHeightInitializer.Initialize(registers.FirstHeight())
			if err != nil {
				return nil, fmt.Errorf("could not initialize indexed block height: %w", err)
			}

			// execution state worker uses a jobqueue to process new execution data and indexes it by using the indexer.
			builder.ExecutionIndexer, err = indexer.NewIndexer(
				builder.Logger,
				registers.FirstHeight(),
				registers,
				builder.ExecutionIndexerCore,
				executionDataStoreCache,
				builder.ExecutionDataRequester.HighestConsecutiveHeight,
				indexedBlockHeight,
			)
			if err != nil {
				return nil, err
			}

			if executionDataPrunerEnabled {
				builder.ExecutionDataPruner.RegisterHeightRecorder(builder.ExecutionIndexer)
			}

			// setup requester to notify indexer when new execution data is received
			execDataDistributor.AddOnExecutionDataReceivedConsumer(builder.ExecutionIndexer.OnExecutionData)

			err = builder.Reporter.Initialize(builder.ExecutionIndexer)
			if err != nil {
				return nil, err
			}

			// create script execution module, this depends on the indexer being initialized and the
			// having the register storage bootstrapped
			scripts := execution.NewScripts(
				builder.Logger,
				metrics.NewExecutionCollector(builder.Tracer),
				builder.RootChainID,
				computation.NewProtocolStateWrapper(builder.State),
				builder.Storage.Headers,
				builder.ExecutionIndexerCore.RegisterValue,
				builder.scriptExecutorConfig,
				queryDerivedChainData,
				builder.programCacheSize > 0,
			)

			err = builder.ScriptExecutor.Initialize(builder.ExecutionIndexer, scripts, builder.VersionControl)
			if err != nil {
				return nil, err
			}

			err = builder.RegistersAsyncStore.Initialize(registers)
			if err != nil {
				return nil, err
			}

			if builder.stopControlEnabled {
				builder.StopControl.RegisterHeightRecorder(builder.ExecutionIndexer)
			}

			return builder.ExecutionIndexer, nil
		}, builder.IndexerDependencies)
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
				case "AccountAddresses":
					builder.stateStreamConf.MaxAccountAddress = value
				}
			}
			builder.stateStreamConf.RpcMetricsEnabled = builder.rpcMetricsEnabled

			highestAvailableHeight, err := builder.ExecutionDataRequester.HighestConsecutiveHeight()
			if err != nil {
				return nil, fmt.Errorf("could not get highest consecutive height: %w", err)
			}
			broadcaster := engine.NewBroadcaster()

			eventQueryMode, err := query_mode.ParseIndexQueryMode(builder.rpcConf.BackendConfig.EventQueryMode)
			if err != nil {
				return nil, fmt.Errorf("could not parse event query mode: %w", err)
			}

			// use the events index for events if enabled and the node is configured to use it for
			// regular event queries
			useIndex := builder.executionDataIndexingEnabled &&
				eventQueryMode != query_mode.IndexQueryModeExecutionNodesOnly

			executionDataTracker := subscriptiontracker.NewExecutionDataTracker(
				builder.Logger,
				node.State,
				builder.executionDataConfig.InitialBlockHeight,
				node.Storage.Headers,
				broadcaster,
				highestAvailableHeight,
				builder.EventsIndex,
				useIndex,
			)

			builder.stateStreamBackend, err = statestreambackend.New(
				node.Logger,
				node.State,
				node.Storage.Headers,
				node.Storage.Seals,
				node.Storage.Results,
				builder.ExecutionDataStore,
				executionDataStoreCache,
				builder.RegistersAsyncStore,
				builder.EventsIndex,
				useIndex,
				int(builder.stateStreamConf.RegisterIDsRequestLimit),
				subscription.NewSubscriptionHandler(
					builder.Logger,
					broadcaster,
					builder.stateStreamConf.ClientSendTimeout,
					builder.stateStreamConf.ResponseLimit,
					builder.stateStreamConf.ClientSendBufferSize,
				),
				executionDataTracker,
			)
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
			)
			if err != nil {
				return nil, fmt.Errorf("could not create state stream engine: %w", err)
			}
			builder.StateStreamEng = stateStreamEng

			// setup requester to notify ExecutionDataTracker when new execution data is received
			execDataDistributor.AddOnExecutionDataReceivedConsumer(builder.stateStreamBackend.OnExecutionData)

			return builder.StateStreamEng, nil
		})
	}
	return builder
}

// buildDerivedChainData creates the derived chain data for the indexer and the query engine
// If program caching is disabled, the function will return nil for the indexer cache, and a
// derived chain data object for the query engine cache.
func (builder *ObserverServiceBuilder) buildDerivedChainData() (
	indexerCache *derived.DerivedChainData,
	queryCache *derived.DerivedChainData,
	err error,
) {
	cacheSize := builder.programCacheSize

	// the underlying cache requires size > 0. no data will be written so 1 is fine.
	if cacheSize == 0 {
		cacheSize = 1
	}

	derivedChainData, err := derived.NewDerivedChainData(cacheSize)
	if err != nil {
		return nil, nil, err
	}

	// writes are done by the indexer. using a nil value effectively disables writes to the cache.
	if builder.programCacheSize == 0 {
		return nil, derivedChainData, nil
	}

	return derivedChainData, derivedChainData, nil
}

// enqueuePublicNetworkInit enqueues the observer network component initialized for the observer
func (builder *ObserverServiceBuilder) enqueuePublicNetworkInit() {
	var publicLibp2pNode p2p.LibP2PNode
	builder.
		Component("public libp2p node", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error
			publicLibp2pNode, err = builder.BuildPublicLibp2pNode(builder.BaseConfig.BindAddr, builder.bootstrapIdentities)
			if err != nil {
				return nil, fmt.Errorf("could not build public libp2p node: %w", err)
			}

			builder.LibP2PNode = publicLibp2pNode

			return publicLibp2pNode, nil
		}).
		Component("public network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			receiveCache := netcache.NewHeroReceiveCache(builder.FlowConfig.NetworkConfig.NetworkReceivedMessageCacheSize,
				builder.Logger,
				metrics.NetworkReceiveCacheMetricsFactory(builder.HeroCacheMetricsFactory(), network.PublicNetwork))

			err := node.Metrics.Mempool.Register(metrics.PrependPublicPrefix(metrics.ResourceNetworkingReceiveCache), receiveCache.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register networking receive cache metric: %w", err)
			}

			net, err := underlay.NewNetwork(&underlay.NetworkConfig{
				Logger:                builder.Logger.With().Str("component", "public-network").Logger(),
				Codec:                 builder.CodecFactory(),
				Me:                    builder.Me,
				Topology:              nil, // topology is nil since it is managed by libp2p; //TODO: can we set empty topology?
				Libp2pNode:            publicLibp2pNode,
				Metrics:               builder.Metrics.Network,
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
					HeroCacheMetricsFactory: builder.HeroCacheMetricsFactory(),
					NetworkType:             network.PublicNetwork,
				},
				SlashingViolationConsumerFactory: func(adapter network.ConduitAdapter) network.ViolationsConsumer {
					return slashing.NewSlashingViolationsConsumer(builder.Logger, builder.Metrics.Network, adapter)
				},
			}, underlay.WithMessageValidators(publicNetworkMsgValidators(node.Logger, node.IdentityProvider, node.NodeID)...))
			if err != nil {
				return nil, fmt.Errorf("could not initialize network: %w", err)
			}

			builder.NetworkUnderlay = net
			builder.EngineRegistry = converter.NewNetwork(net, channels.SyncCommittee, channels.PublicSyncCommittee)

			builder.Logger.Info().Msgf("network will run on address: %s", builder.BindAddr)

			idEvents := gadgets.NewIdentityDeltas(builder.NetworkUnderlay.UpdateNodeAddresses)
			builder.ProtocolEvents.AddConsumer(idEvents)

			return builder.EngineRegistry, nil
		})
}

// enqueueConnectWithStakedAN enqueues the upstream connector component which connects the libp2p host of the observer
// service with the AN.
// Currently, there is an issue with LibP2P stopping advertisements of subscribed topics if no peers are connected
// (https://github.com/libp2p/go-libp2p-pubsub/issues/442). This means that an observer could end up not being
// discovered by other observers if it subscribes to a topic before connecting to the AN. Hence, the need
// of an explicit connect to the AN before the node attempts to subscribe to topics.
func (builder *ObserverServiceBuilder) enqueueConnectWithStakedAN() {
	builder.Component("upstream connector", func(_ *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return consensus_follower.NewUpstreamConnector(builder.bootstrapIdentities, builder.LibP2PNode, builder.Logger), nil
	})
}

func (builder *ObserverServiceBuilder) enqueueRPCServer() {
	builder.Module("transaction metrics", func(node *cmd.NodeConfig) error {
		builder.TransactionTimings = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
		builder.TransactionMetrics = metrics.NewTransactionCollector(
			node.Logger,
			builder.TransactionTimings,
			builder.logTxTimeToFinalized,
			builder.logTxTimeToExecuted,
			builder.logTxTimeToFinalizedExecuted,
			builder.logTxTimeToSealed,
		)
		return nil
	})
	builder.Module("rest metrics", func(node *cmd.NodeConfig) error {
		m, err := metrics.NewRestCollector(router.MethodURLToRoute, node.MetricsRegisterer)
		if err != nil {
			return err
		}
		builder.RestMetrics = m
		return nil
	})
	builder.Module("access metrics", func(node *cmd.NodeConfig) error {
		builder.AccessMetrics = metrics.NewAccessCollector(
			metrics.WithTransactionMetrics(builder.TransactionMetrics),
			metrics.WithBackendScriptsMetrics(builder.TransactionMetrics),
			metrics.WithRestMetrics(builder.RestMetrics),
		)
		return nil
	})
	builder.Module("server certificate", func(node *cmd.NodeConfig) error {
		// generate the server certificate that will be served by the GRPC server
		x509Certificate, err := grpcutils.X509Certificate(node.NetworkKey)
		if err != nil {
			return err
		}
		tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
		builder.rpcConf.TransportCredentials = credentials.NewTLS(tlsConfig)
		return nil
	})
	builder.Module("creating grpc servers", func(node *cmd.NodeConfig) error {
		if builder.rpcConf.DeprecatedMaxMsgSize != 0 {
			node.Logger.Warn().Msg("A deprecated flag was specified (--rpc-max-message-size). Use --rpc-max-request-message-size and --rpc-max-response-message-size instead. This flag will be removed in a future release.")
			builder.rpcConf.BackendConfig.AccessConfig.MaxRequestMsgSize = builder.rpcConf.DeprecatedMaxMsgSize
			builder.rpcConf.BackendConfig.AccessConfig.MaxResponseMsgSize = builder.rpcConf.DeprecatedMaxMsgSize
		}

		builder.secureGrpcServer = grpcserver.NewGrpcServerBuilder(node.Logger,
			builder.rpcConf.SecureGRPCListenAddr,
			builder.rpcConf.BackendConfig.AccessConfig.MaxRequestMsgSize,
			builder.rpcConf.BackendConfig.AccessConfig.MaxResponseMsgSize,
			builder.rpcMetricsEnabled,
			builder.apiRatelimits,
			builder.apiBurstlimits,
			grpcserver.WithTransportCredentials(builder.rpcConf.TransportCredentials)).Build()

		builder.stateStreamGrpcServer = grpcserver.NewGrpcServerBuilder(
			node.Logger,
			builder.stateStreamConf.ListenAddr,
			builder.rpcConf.BackendConfig.AccessConfig.MaxRequestMsgSize,
			builder.stateStreamConf.MaxExecutionDataMsgSize,
			builder.rpcMetricsEnabled,
			builder.apiRatelimits,
			builder.apiBurstlimits,
			grpcserver.WithStreamInterceptor()).Build()

		if builder.rpcConf.UnsecureGRPCListenAddr != builder.stateStreamConf.ListenAddr {
			builder.unsecureGrpcServer = grpcserver.NewGrpcServerBuilder(node.Logger,
				builder.rpcConf.UnsecureGRPCListenAddr,
				builder.rpcConf.BackendConfig.AccessConfig.MaxRequestMsgSize,
				builder.rpcConf.BackendConfig.AccessConfig.MaxResponseMsgSize,
				builder.rpcMetricsEnabled,
				builder.apiRatelimits,
				builder.apiBurstlimits).Build()
		} else {
			builder.unsecureGrpcServer = builder.stateStreamGrpcServer
		}

		return nil
	})
	builder.Module("async register store", func(node *cmd.NodeConfig) error {
		builder.RegistersAsyncStore = execution.NewRegistersAsyncStore()
		return nil
	})
	builder.Module("events storage", func(node *cmd.NodeConfig) error {
		builder.events = store.NewEvents(node.Metrics.Cache, node.ProtocolDB)
		return nil
	})
	builder.Module("reporter", func(node *cmd.NodeConfig) error {
		builder.Reporter = index.NewReporter()
		return nil
	})
	builder.Module("events index", func(node *cmd.NodeConfig) error {
		builder.EventsIndex = index.NewEventsIndex(builder.Reporter, builder.events)
		return nil
	})
	builder.Module("transaction result index", func(node *cmd.NodeConfig) error {
		builder.TxResultsIndex = index.NewTransactionResultsIndex(builder.Reporter, builder.lightTransactionResults)
		return nil
	})
	builder.Module("script executor", func(node *cmd.NodeConfig) error {
		builder.ScriptExecutor = backend.NewScriptExecutor(builder.Logger, builder.scriptExecMinBlock, builder.scriptExecMaxBlock)
		return nil
	})

	versionControlDependable := module.NewProxiedReadyDoneAware()
	builder.IndexerDependencies.Add(versionControlDependable)
	stopControlDependable := module.NewProxiedReadyDoneAware()
	builder.IndexerDependencies.Add(stopControlDependable)

	builder.Component("version control", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		if !builder.versionControlEnabled {
			noop := &module.NoopReadyDoneAware{}
			versionControlDependable.Init(noop)
			return noop, nil
		}

		nodeVersion, err := build.Semver()
		if err != nil {
			return nil, fmt.Errorf("could not load node version for version control. "+
				"version (%s) is not semver compliant: %w. Make sure a valid semantic version is provided in the VERSION environment variable", build.Version(), err)
		}

		versionControl, err := version.NewVersionControl(
			builder.Logger,
			node.Storage.VersionBeacons,
			nodeVersion,
			builder.SealedRootBlock.Height,
			builder.LastFinalizedHeader.Height,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create version control: %w", err)
		}

		// VersionControl needs to consume BlockFinalized events.
		node.ProtocolEvents.AddConsumer(versionControl)

		builder.VersionControl = versionControl
		versionControlDependable.Init(builder.VersionControl)

		return versionControl, nil
	})
	builder.Component("stop control", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		if !builder.stopControlEnabled {
			noop := &module.NoopReadyDoneAware{}
			stopControlDependable.Init(noop)
			return noop, nil
		}

		stopControl := stop.NewStopControl(
			builder.Logger,
		)

		builder.VersionControl.AddVersionUpdatesConsumer(stopControl.OnVersionUpdate)

		builder.StopControl = stopControl
		stopControlDependable.Init(builder.StopControl)

		return stopControl, nil
	})

	builder.Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		accessMetrics := builder.AccessMetrics
		config := builder.rpcConf
		backendConfig := config.BackendConfig
		cacheSize := int(backendConfig.ConnectionPoolSize)

		var connBackendCache *rpcConnection.Cache
		var err error
		if cacheSize > 0 {
			connBackendCache, err = rpcConnection.NewCache(node.Logger, accessMetrics, cacheSize)
			if err != nil {
				return nil, fmt.Errorf("could not initialize connection cache: %w", err)
			}
		}

		connFactory := &rpcConnection.ConnectionFactoryImpl{
			AccessConfig:     backendConfig.AccessConfig,
			CollectionConfig: backendConfig.CollectionConfig,
			ExecutionConfig:  backendConfig.ExecutionConfig,
			AccessMetrics:    accessMetrics,
			Log:              node.Logger,
			Manager: rpcConnection.NewManager(
				node.Logger,
				accessMetrics,
				connBackendCache,
				backendConfig.CircuitBreakerConfig,
				config.CompressorName,
			),
		}

		broadcaster := engine.NewBroadcaster()
		// create BlockTracker that will track for new blocks (finalized and sealed) and
		// handles block-related operations.
		blockTracker, err := subscriptiontracker.NewBlockTracker(
			node.State,
			builder.SealedRootBlock.Height,
			node.Storage.Headers,
			broadcaster,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize block tracker: %w", err)
		}

		// If execution data syncing and indexing is disabled, pass nil indexReporter
		var indexReporter state_synchronization.IndexReporter
		if builder.executionDataSyncEnabled && builder.executionDataIndexingEnabled {
			indexReporter = builder.Reporter
		}

		preferredENIdentifiers, err := flow.IdentifierListFromHex(backendConfig.PreferredExecutionNodeIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for preferred EN map: %w", err)
		}

		fixedENIdentifiers, err := flow.IdentifierListFromHex(backendConfig.FixedExecutionNodeIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for fixed EN map: %w", err)
		}

		scriptExecMode, err := query_mode.ParseIndexQueryMode(config.BackendConfig.ScriptExecutionMode)
		if err != nil {
			return nil, fmt.Errorf("could not parse script execution mode: %w", err)
		}

		eventQueryMode, err := query_mode.ParseIndexQueryMode(config.BackendConfig.EventQueryMode)
		if err != nil {
			return nil, fmt.Errorf("could not parse event query mode: %w", err)
		}
		if eventQueryMode == query_mode.IndexQueryModeCompare {
			return nil, fmt.Errorf("event query mode 'compare' is not supported")
		}

		txResultQueryMode, err := query_mode.ParseIndexQueryMode(config.BackendConfig.TxResultQueryMode)
		if err != nil {
			return nil, fmt.Errorf("could not parse transaction result query mode: %w", err)
		}
		if txResultQueryMode == query_mode.IndexQueryModeCompare {
			return nil, fmt.Errorf("transaction result query mode 'compare' is not supported")
		}

		execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
			node.Logger,
			node.State,
			node.Storage.Receipts,
			preferredENIdentifiers,
			fixedENIdentifiers,
		)

		backendParams := backend.Params{
			State:                 node.State,
			Blocks:                node.Storage.Blocks,
			Headers:               node.Storage.Headers,
			Collections:           node.Storage.Collections,
			Transactions:          node.Storage.Transactions,
			ExecutionReceipts:     node.Storage.Receipts,
			ExecutionResults:      node.Storage.Results,
			Seals:                 node.Storage.Seals,
			ScheduledTransactions: builder.scheduledTransactions,
			ChainID:               node.RootChainID,
			AccessMetrics:         accessMetrics,
			ConnFactory:           connFactory,
			MaxHeightRange:        backendConfig.MaxHeightRange,
			Log:                   node.Logger,
			SnapshotHistoryLimit:  backend.DefaultSnapshotHistoryLimit,
			Communicator:          node_communicator.NewNodeCommunicator(backendConfig.CircuitBreakerConfig.Enabled),
			BlockTracker:          blockTracker,
			ScriptExecutionMode:   scriptExecMode,
			EventQueryMode:        eventQueryMode,
			TxResultQueryMode:     txResultQueryMode,
			SubscriptionHandler: subscription.NewSubscriptionHandler(
				builder.Logger,
				broadcaster,
				builder.stateStreamConf.ClientSendTimeout,
				builder.stateStreamConf.ResponseLimit,
				builder.stateStreamConf.ClientSendBufferSize,
			),
			IndexReporter:              indexReporter,
			VersionControl:             builder.VersionControl,
			ExecNodeIdentitiesProvider: execNodeIdentitiesProvider,
			MaxScriptAndArgumentSize:   config.BackendConfig.AccessConfig.MaxRequestMsgSize,
		}

		if builder.localServiceAPIEnabled {
			backendParams.ScriptExecutionMode = query_mode.IndexQueryModeLocalOnly
			backendParams.EventQueryMode = query_mode.IndexQueryModeLocalOnly
			backendParams.TxResultsIndex = builder.TxResultsIndex
			backendParams.EventsIndex = builder.EventsIndex
			backendParams.ScriptExecutor = builder.ScriptExecutor
		}

		accessBackend, err := backend.New(backendParams)
		if err != nil {
			return nil, fmt.Errorf("could not initialize backend: %w", err)
		}

		observerCollector := metrics.NewObserverCollector()
		restHandler, err := restapiproxy.NewRestProxyHandler(
			accessBackend,
			builder.upstreamIdentities,
			connFactory,
			builder.Logger,
			observerCollector,
			node.RootChainID.Chain())
		if err != nil {
			return nil, err
		}

		engineBuilder, err := rpc.NewBuilder(
			node.Logger,
			node.State,
			config,
			node.RootChainID,
			accessMetrics,
			builder.rpcMetricsEnabled,
			builder.Me,
			accessBackend,
			restHandler,
			builder.secureGrpcServer,
			builder.unsecureGrpcServer,
			builder.stateStreamBackend,
			builder.stateStreamConf,
			indexReporter,
			builder.FollowerDistributor,
		)
		if err != nil {
			return nil, err
		}

		// upstream access node forwarder
		forwarder, err := apiproxy.NewFlowAccessAPIForwarder(builder.upstreamIdentities, connFactory)
		if err != nil {
			return nil, err
		}

		rpcHandler := apiproxy.NewFlowAccessAPIRouter(apiproxy.Params{
			Log:      builder.Logger,
			Metrics:  observerCollector,
			Upstream: forwarder,
			Local:    engineBuilder.DefaultHandler(hotsignature.NewBlockSignerDecoder(builder.Committee)),
			UseIndex: builder.localServiceAPIEnabled,
		})

		// build the rpc engine
		builder.RpcEng, err = engineBuilder.
			WithRpcHandler(rpcHandler).
			WithLegacy().
			Build()
		if err != nil {
			return nil, err
		}
		return builder.RpcEng, nil
	})

	// build secure grpc server
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
}

func loadNetworkingKey(path string) (crypto.PrivateKey, error) {
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read networking key (path=%s): %w", path, err)
	}

	keyBytes, err := hex.DecodeString(strings.Trim(string(data), "\n "))
	if err != nil {
		return nil, fmt.Errorf("could not hex decode networking key (path=%s): %w", path, err)
	}

	networkingKey, err := crypto.DecodePrivateKey(crypto.ECDSASecp256k1, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not decode networking key (path=%s): %w", path, err)
	}

	return networkingKey, nil
}
