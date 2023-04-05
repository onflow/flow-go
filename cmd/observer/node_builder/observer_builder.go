package node_builder

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	badger "github.com/ipfs/go-ds-badger2"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/go-bitswap"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/access/apiproxy"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/follower"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/protocol"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	edrequester "github.com/onflow/flow-go/module/state_synchronization/requester"
	consensus_follower "github.com/onflow/flow-go/module/upstream"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/converter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/p2p/cache"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/validator"
	stateprotocol "github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
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
	bootstrapNodeAddresses    []string
	bootstrapNodePublicKeys   []string
	observerNetworkingKeyPath string
	bootstrapIdentities       flow.IdentityList // the identity list of bootstrap peers the node uses to discover other nodes
	apiRatelimits             map[string]int
	apiBurstlimits            map[string]int
	rpcConf                   rpc.Config
	rpcMetricsEnabled         bool
	executionDataSyncEnabled  bool
	executionDataDir          string
	executionDataStartHeight  uint64
	executionDataConfig       edrequester.ExecutionDataConfig
	apiTimeout                time.Duration
	upstreamNodeAddresses     []string
	upstreamNodePublicKeys    []string
	upstreamIdentities        flow.IdentityList // the identity list of upstream peers the node uses to forward API requests to
}

// DefaultObserverServiceConfig defines all the default values for the ObserverServiceConfig
func DefaultObserverServiceConfig() *ObserverServiceConfig {
	homedir, _ := os.UserHomeDir()
	return &ObserverServiceConfig{
		rpcConf: rpc.Config{
			UnsecureGRPCListenAddr:    "0.0.0.0:9000",
			SecureGRPCListenAddr:      "0.0.0.0:9001",
			HTTPListenAddr:            "0.0.0.0:8000",
			RESTListenAddr:            "",
			CollectionAddr:            "",
			HistoricalAccessAddrs:     "",
			CollectionClientTimeout:   3 * time.Second,
			ExecutionClientTimeout:    3 * time.Second,
			MaxHeightRange:            backend.DefaultMaxHeightRange,
			PreferredExecutionNodeIDs: nil,
			FixedExecutionNodeIDs:     nil,
			MaxMsgSize:                grpcutils.DefaultMaxMsgSize,
		},
		rpcMetricsEnabled:         false,
		apiRatelimits:             nil,
		apiBurstlimits:            nil,
		bootstrapNodeAddresses:    []string{},
		bootstrapNodePublicKeys:   []string{},
		observerNetworkingKeyPath: cmd.NotSet,
		executionDataSyncEnabled:  false,
		executionDataDir:          filepath.Join(homedir, ".flow", "execution_data"),
		executionDataStartHeight:  0,
		executionDataConfig: edrequester.ExecutionDataConfig{
			InitialBlockHeight: 0,
			MaxSearchAhead:     edrequester.DefaultMaxSearchAhead,
			FetchTimeout:       edrequester.DefaultFetchTimeout,
			RetryDelay:         edrequester.DefaultRetryDelay,
			MaxRetryDelay:      edrequester.DefaultMaxRetryDelay,
		},
		apiTimeout:             3 * time.Second,
		upstreamNodeAddresses:  []string{},
		upstreamNodePublicKeys: []string{},
	}
}

// ObserverServiceBuilder provides the common functionality needed to bootstrap a Flow observer service
// It is composed of the FlowNodeBuilder, the ObserverServiceConfig and contains all the components and modules needed for the observers
type ObserverServiceBuilder struct {
	*cmd.FlowNodeBuilder
	*ObserverServiceConfig

	// components
	LibP2PNode              p2p.LibP2PNode
	FollowerState           stateprotocol.FollowerState
	SyncCore                *chainsync.Core
	RpcEng                  *rpc.Engine
	FinalizationDistributor *pubsub.FinalizationDistributor
	FinalizedHeader         *synceng.FinalizedHeaderCache
	Committee               hotstuff.DynamicCommittee
	Finalized               *flow.Header
	Pending                 []*flow.Header
	FollowerCore            module.HotStuffFollower
	Validator               hotstuff.Validator
	ExecutionDataDownloader execution_data.Downloader
	ExecutionDataRequester  state_synchronization.ExecutionDataRequester // for the observer, the sync engine participants provider is the libp2p peer store which is not
	// available until after the network has started. Hence, a factory function that needs to be called just before
	// creating the sync engine
	SyncEngineParticipantsProviderFactory func() module.IdentifierProvider

	// engines
	FollowerEng *follower.ComplianceEngine
	SyncEng     *synceng.Engine

	// Public network
	peerID peer.ID
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstrap peers from the parameters.
// These are the identities of the observers also acting as the DHT bootstrap server
func (builder *ObserverServiceBuilder) deriveBootstrapPeerIdentities() error {
	// if bootstrap identities already provided (as part of alternate initialization as a library the skip reading command
	// line params)
	if builder.bootstrapIdentities != nil {
		return nil
	}

	ids, err := BootstrapIdentities(builder.bootstrapNodeAddresses, builder.bootstrapNodePublicKeys)
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

	ids := make([]*flow.Identity, len(addresses))
	for i, address := range addresses {
		key := keys[i]

		// json unmarshaller needs a quotes before and after the string
		// the pflags.StringSliceVar does not retain quotes for the command line arg even if escaped with \"
		// hence this additional check to ensure the key is indeed quoted
		if !strings.HasPrefix(key, "\"") {
			key = fmt.Sprintf("\"%s\"", key)
		}

		// create the identity of the peer by setting only the relevant fields
		ids[i] = &flow.Identity{
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
		final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, builder.FollowerState, node.Tracer)

		packer := hotsignature.NewConsensusSigDataPacker(builder.Committee)
		// initialize the verifier for the protocol consensus
		verifier := verification.NewCombinedVerifier(builder.Committee, packer)
		builder.Validator = hotstuffvalidator.New(builder.Committee, verifier)

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

func (builder *ObserverServiceBuilder) buildFollowerEngine() *ObserverServiceBuilder {
	builder.Component("follower engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
		if node.HeroCacheMetricsEnable {
			heroCacheCollector = metrics.FollowerCacheMetrics(node.MetricsRegisterer)
		}
		core, err := follower.NewComplianceCore(
			node.Logger,
			node.Metrics.Mempool,
			heroCacheCollector,
			builder.FinalizationDistributor,
			builder.FollowerState,
			builder.FollowerCore,
			builder.Validator,
			builder.SyncCore,
			node.Tracer,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower core: %w", err)
		}

		builder.FollowerEng, err = follower.NewComplianceLayer(
			node.Logger,
			node.Network,
			node.Me,
			node.Metrics.Engine,
			node.Storage.Headers,
			builder.Finalized,
			core,
			follower.WithComplianceConfigOpt(compliance.WithSkipNewProposalsThreshold(builder.ComplianceConfig.SkipNewProposalsThreshold)),
			follower.WithChannel(channels.PublicReceiveBlocks),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}

		return builder.FollowerEng, nil
	})

	return builder
}

func (builder *ObserverServiceBuilder) buildFinalizedHeader() *ObserverServiceBuilder {
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

func (builder *ObserverServiceBuilder) buildSyncEngine() *ObserverServiceBuilder {
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

func (builder *ObserverServiceBuilder) BuildConsensusFollower() cmd.NodeBuilder {
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

func (builder *ObserverServiceBuilder) BuildExecutionDataRequester() *ObserverServiceBuilder {
	var ds *badger.Datastore
	var bs network.BlobService
	var processedBlockHeight storage.ConsumerProgress
	var processedNotifications storage.ConsumerProgress

	builder.
		Module("execution data datastore and blobstore", func(node *cmd.NodeConfig) error {
			err := os.MkdirAll(builder.executionDataDir, 0700)
			if err != nil {
				return err
			}

			ds, err = badger.NewDatastore(builder.executionDataDir, &badger.DefaultOptions)
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
		Component("execution data service", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error
			bs, err = node.Network.RegisterBlobService(channels.ExecutionDataService, ds,
				blob.WithBitswapOptions(
					bitswap.WithTracer(
						blob.NewTracer(node.Logger.With().Str("blob_service", channels.ExecutionDataService.String()).Logger()),
					),
				),
			)
			if err != nil {
				return nil, fmt.Errorf("could not register blob service: %w", err)
			}

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

type Option func(*ObserverServiceConfig)

func NewFlowObserverServiceBuilder(opts ...Option) *ObserverServiceBuilder {
	config := DefaultObserverServiceConfig()
	for _, opt := range opts {
		opt(config)
	}
	anb := &ObserverServiceBuilder{
		ObserverServiceConfig:   config,
		FlowNodeBuilder:         cmd.FlowNode(flow.RoleAccess.String()),
		FinalizationDistributor: pubsub.NewFinalizationDistributor(),
	}
	anb.FinalizationDistributor.AddConsumer(notifications.NewSlashingViolationsConsumer(anb.Logger))
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

		flags.StringVarP(&builder.rpcConf.UnsecureGRPCListenAddr, "rpc-addr", "r", defaultConfig.rpcConf.UnsecureGRPCListenAddr, "the address the unsecured gRPC server listens on")
		flags.StringVar(&builder.rpcConf.SecureGRPCListenAddr, "secure-rpc-addr", defaultConfig.rpcConf.SecureGRPCListenAddr, "the address the secure gRPC server listens on")
		flags.StringVarP(&builder.rpcConf.HTTPListenAddr, "http-addr", "h", defaultConfig.rpcConf.HTTPListenAddr, "the address the http proxy server listens on")
		flags.StringVar(&builder.rpcConf.RESTListenAddr, "rest-addr", defaultConfig.rpcConf.RESTListenAddr, "the address the REST server listens on (if empty the REST server will not be started)")
		flags.UintVar(&builder.rpcConf.MaxMsgSize, "rpc-max-message-size", defaultConfig.rpcConf.MaxMsgSize, "the maximum message size in bytes for messages sent or received over grpc")
		flags.UintVar(&builder.rpcConf.MaxHeightRange, "rpc-max-height-range", defaultConfig.rpcConf.MaxHeightRange, "maximum size for height range requests")
		flags.StringToIntVar(&builder.apiRatelimits, "api-rate-limits", defaultConfig.apiRatelimits, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&builder.apiBurstlimits, "api-burst-limits", defaultConfig.apiBurstlimits, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.StringVar(&builder.observerNetworkingKeyPath, "observer-networking-key-path", defaultConfig.observerNetworkingKeyPath, "path to the networking key for observer")
		flags.StringSliceVar(&builder.bootstrapNodeAddresses, "bootstrap-node-addresses", defaultConfig.bootstrapNodeAddresses, "the network addresses of the bootstrap access node if this is an observer e.g. access-001.mainnet.flow.org:9653,access-002.mainnet.flow.org:9653")
		flags.StringSliceVar(&builder.bootstrapNodePublicKeys, "bootstrap-node-public-keys", defaultConfig.bootstrapNodePublicKeys, "the networking public key of the bootstrap access node if this is an observer (in the same order as the bootstrap node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")
		flags.DurationVar(&builder.apiTimeout, "upstream-api-timeout", defaultConfig.apiTimeout, "tcp timeout for Flow API gRPC sockets to upstrem nodes")
		flags.StringSliceVar(&builder.upstreamNodeAddresses, "upstream-node-addresses", defaultConfig.upstreamNodeAddresses, "the gRPC network addresses of the upstream access node. e.g. access-001.mainnet.flow.org:9000,access-002.mainnet.flow.org:9000")
		flags.StringSliceVar(&builder.upstreamNodePublicKeys, "upstream-node-public-keys", defaultConfig.upstreamNodePublicKeys, "the networking public key of the upstream access node (in the same order as the upstream node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")
		flags.BoolVar(&builder.rpcMetricsEnabled, "rpc-metrics-enabled", defaultConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")

		// ExecutionDataRequester config
		flags.BoolVar(&builder.executionDataSyncEnabled, "execution-data-sync-enabled", defaultConfig.executionDataSyncEnabled, "whether to enable the execution data sync protocol")
		flags.StringVar(&builder.executionDataDir, "execution-data-dir", defaultConfig.executionDataDir, "directory to use for Execution Data database")
		flags.Uint64Var(&builder.executionDataStartHeight, "execution-data-start-height", defaultConfig.executionDataStartHeight, "height of first block to sync execution data from when starting with an empty Execution Data database")
		flags.Uint64Var(&builder.executionDataConfig.MaxSearchAhead, "execution-data-max-search-ahead", defaultConfig.executionDataConfig.MaxSearchAhead, "max number of heights to search ahead of the lowest outstanding execution data height")
		flags.DurationVar(&builder.executionDataConfig.FetchTimeout, "execution-data-fetch-timeout", defaultConfig.executionDataConfig.FetchTimeout, "timeout to use when fetching execution data from the network e.g. 300s")
		flags.DurationVar(&builder.executionDataConfig.RetryDelay, "execution-data-retry-delay", defaultConfig.executionDataConfig.RetryDelay, "initial delay for exponential backoff when fetching execution data fails e.g. 10s")
		flags.DurationVar(&builder.executionDataConfig.MaxRetryDelay, "execution-data-max-retry-delay", defaultConfig.executionDataConfig.MaxRetryDelay, "maximum delay for exponential backoff when fetching execution data fails e.g. 5m")
	}).ValidateFlags(func() error {
		if builder.executionDataSyncEnabled {
			if builder.executionDataConfig.FetchTimeout <= 0 {
				return errors.New("execution-data-fetch-timeout must be greater than 0")
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
func (builder *ObserverServiceBuilder) initNetwork(nodeID module.Local,
	networkMetrics module.NetworkCoreMetrics,
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

// BootstrapIdentities converts the bootstrap node addresses and keys to a Flow Identity list where
// each Flow Identity is initialized with the passed address, the networking key
// and the Node ID set to ZeroID, role set to Access, 0 stake and no staking key.
func BootstrapIdentities(addresses []string, keys []string) (flow.IdentityList, error) {
	if len(addresses) != len(keys) {
		return nil, fmt.Errorf("number of addresses and keys provided for the boostrap nodes don't match")
	}

	ids := make([]*flow.Identity, len(addresses))
	for i, address := range addresses {
		bytes, err := hex.DecodeString(keys[i])
		if err != nil {
			return nil, fmt.Errorf("failed to decode secured GRPC server public key hex %w", err)
		}

		publicFlowNetworkingKey, err := crypto.DecodePublicKey(crypto.ECDSAP256, bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to get public flow networking key could not decode public key bytes %w", err)
		}

		// create the identity of the peer by setting only the relevant fields
		ids[i] = &flow.Identity{
			NodeID:        flow.ZeroID, // the NodeID is the hash of the staking key and for the public network it does not apply
			Address:       address,
			Role:          flow.RoleAccess, // the upstream node has to be an access node
			NetworkPubKey: publicFlowNetworkingKey,
		}
	}
	return ids, nil
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

		builder.NodeDisallowListDistributor = cmd.BuildDisallowListNotificationDisseminator(builder.DisallowListNotificationCacheSize, builder.MetricsRegisterer, builder.Logger, builder.MetricsEnabled)

		// The following wrapper allows to black-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of disallow-listed nodes to true
		builder.IdentityProvider, err = cache.NewNodeBlocklistWrapper(idCache, node.DB, builder.NodeDisallowListDistributor)
		if err != nil {
			return fmt.Errorf("could not initialize NodeBlockListWrapper: %w", err)
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
						builder.Logger.Err(err).Str("peer", pid.String()).Msg("failed to translate to Flow ID")
					} else {
						result = append(result, flowID)
					}
				}

				return result
			})
		}

		return nil
	})

	builder.Component("disallow list notification distributor", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// distributor is returned as a component to be started and stopped.
		return builder.NodeDisallowListDistributor, nil
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

	builder.enqueueRPCServer()

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
	if len(builder.bootstrapNodeAddresses) == 0 {
		return errors.New("no bootstrap node address provided")
	}
	if len(builder.bootstrapNodeAddresses) != len(builder.bootstrapNodePublicKeys) {
		return errors.New("number of bootstrap node addresses and public keys should match")
	}
	if len(builder.upstreamNodePublicKeys) > 0 && len(builder.upstreamNodeAddresses) != len(builder.upstreamNodePublicKeys) {
		return errors.New("number of upstream node addresses and public keys must match if public keys given")
	}
	return nil
}

// initPublicLibP2PFactory creates the LibP2P factory function for the given node ID and network key for the observer.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
// The LibP2P host is created with the following options:
// * DHT as client and seeded with the given bootstrap peers
// * The specified bind address as the listen address
// * The passed in private key as the libp2p key
// * No connection gater
// * No connection manager
// * No peer manager
// * Default libp2p pubsub options
func (builder *ObserverServiceBuilder) initPublicLibP2PFactory(networkKey crypto.PrivateKey) p2p.LibP2PFactoryFunc {
	return func() (p2p.LibP2PNode, error) {
		var pis []peer.AddrInfo

		for _, b := range builder.bootstrapIdentities {
			pi, err := utils.PeerAddressInfo(*b)

			if err != nil {
				return nil, fmt.Errorf("could not extract peer address info from bootstrap identity %v: %w", b, err)
			}

			pis = append(pis, pi)
		}

		meshTracer := tracer.NewGossipSubMeshTracer(
			builder.Logger,
			builder.Metrics.Network,
			builder.IdentityProvider,
			builder.GossipSubConfig.LocalMeshLogInterval)

		rpcInspectorBuilder := inspector.NewGossipSubInspectorBuilder(builder.Logger, builder.SporkID, builder.GossipSubRPCInspectorsConfig, builder.GossipSubInspectorNotifDistributor)
		rpcInspectors, err := rpcInspectorBuilder.
			SetPublicNetwork(p2p.PublicNetworkEnabled).
			SetMetrics(builder.Metrics.Network, builder.MetricsRegisterer).
			SetMetricsEnabled(builder.MetricsEnabled).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create gossipsub rpc inspectors: %w", err)
		}

		node, err := p2pbuilder.NewNodeBuilder(
			builder.Logger,
			builder.Metrics.Network,
			builder.BaseConfig.BindAddr,
			networkKey,
			builder.SporkID,
			builder.LibP2PResourceManagerConfig).
			SetSubscriptionFilter(
				subscription.NewRoleBasedFilter(
					subscription.UnstakedRole, builder.IdentityProvider,
				),
			).
			SetRoutingSystem(func(ctx context.Context, h host.Host) (routing.Routing, error) {
				return p2pdht.NewDHT(ctx, h, protocols.FlowPublicDHTProtocolID(builder.SporkID),
					builder.Logger,
					builder.Metrics.Network,
					p2pdht.AsClient(),
					dht.BootstrapPeers(pis...),
				)
			}).
			SetStreamCreationRetryInterval(builder.UnicastCreateStreamRetryDelay).
			SetGossipSubTracer(meshTracer).
			SetGossipSubScoreTracerInterval(builder.GossipSubConfig.ScoreTracerInterval).
			SetGossipSubRPCInspectors(rpcInspectors...).
			Build()

		if err != nil {
			return nil, err
		}

		builder.LibP2PNode = node

		return builder.LibP2PNode, nil
	}
}

// initObserverLocal initializes the observer's ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (builder *ObserverServiceBuilder) initObserverLocal() func(node *cmd.NodeConfig) error {
	return func(node *cmd.NodeConfig) error {
		// for an observer, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
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
		builder.BuildExecutionDataRequester()
	}
	return builder.FlowNodeBuilder.Build()
}

// enqueuePublicNetworkInit enqueues the observer network component initialized for the observer
func (builder *ObserverServiceBuilder) enqueuePublicNetworkInit() {
	var libp2pNode p2p.LibP2PNode
	builder.
		Component("public libp2p node", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			libP2PFactory := builder.initPublicLibP2PFactory(node.NetworkKey)

			var err error
			libp2pNode, err = libP2PFactory()
			if err != nil {
				return nil, fmt.Errorf("could not create public libp2p node: %w", err)
			}

			return libp2pNode, nil
		}).
		Component("public network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
			if builder.HeroCacheMetricsEnable {
				heroCacheCollector = metrics.NetworkReceiveCacheMetricsFactory(builder.MetricsRegisterer)
			}
			receiveCache := netcache.NewHeroReceiveCache(builder.NetworkReceivedMessageCacheSize,
				builder.Logger,
				heroCacheCollector)

			err := node.Metrics.Mempool.Register(metrics.ResourceNetworkingReceiveCache, receiveCache.Size)
			if err != nil {
				return nil, fmt.Errorf("could not register networking receive cache metric: %w", err)
			}

			msgValidators := publicNetworkMsgValidators(node.Logger, node.IdentityProvider, node.NodeID)

			builder.initMiddleware(node.NodeID, libp2pNode, msgValidators...)

			// topology is nil since it is automatically managed by libp2p
			net, err := builder.initNetwork(builder.Me, builder.Metrics.Network, builder.Middleware, nil, receiveCache)
			if err != nil {
				return nil, err
			}

			builder.Network = converter.NewNetwork(net, channels.SyncCommittee, channels.PublicSyncCommittee)

			builder.Logger.Info().Msgf("network will run on address: %s", builder.BindAddr)

			idEvents := gadgets.NewIdentityDeltas(builder.Middleware.UpdateNodeAddresses)
			builder.ProtocolEvents.AddConsumer(idEvents)

			return builder.Network, nil
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
	builder.Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		engineBuilder, err := rpc.NewBuilder(
			node.Logger,
			node.State,
			builder.rpcConf,
			nil,
			nil,
			node.Storage.Blocks,
			node.Storage.Headers,
			node.Storage.Collections,
			node.Storage.Transactions,
			node.Storage.Receipts,
			node.Storage.Results,
			node.RootChainID,
			nil,
			nil,
			0,
			0,
			false,
			builder.rpcMetricsEnabled,
			builder.apiRatelimits,
			builder.apiBurstlimits,
		)
		if err != nil {
			return nil, err
		}

		// upstream access node forwarder
		forwarder, err := apiproxy.NewFlowAccessAPIForwarder(builder.upstreamIdentities, builder.apiTimeout, builder.rpcConf.MaxMsgSize)
		if err != nil {
			return nil, err
		}

		proxy := &apiproxy.FlowAccessAPIRouter{
			Logger:   builder.Logger,
			Metrics:  metrics.NewObserverCollector(),
			Upstream: forwarder,
			Observer: protocol.NewHandler(protocol.New(
				node.State,
				node.Storage.Blocks,
				node.Storage.Headers,
				backend.NewNetworkAPI(node.State, node.RootChainID, backend.DefaultSnapshotHistoryLimit),
			)),
		}

		// build the rpc engine
		builder.RpcEng, err = engineBuilder.
			WithNewHandler(proxy).
			WithLegacy().
			Build()
		if err != nil {
			return nil, err
		}
		return builder.RpcEng, nil
	})
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (builder *ObserverServiceBuilder) initMiddleware(nodeID flow.Identifier,
	libp2pNode p2p.LibP2PNode,
	validators ...network.MessageValidator,
) network.Middleware {
	slashingViolationsConsumer := slashing.NewSlashingViolationsConsumer(builder.Logger, builder.Metrics.Network)
	mw := middleware.NewMiddleware(
		builder.Logger,
		libp2pNode, nodeID,
		builder.Metrics.Bitswap,
		builder.SporkID,
		middleware.DefaultUnicastTimeout,
		builder.IDTranslator,
		builder.CodecFactory(),
		slashingViolationsConsumer,
		middleware.WithMessageValidators(validators...), // use default identifier provider
	)
	builder.NodeDisallowListDistributor.AddConsumer(mw)
	builder.Middleware = mw
	return builder.Middleware
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
