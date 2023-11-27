package node_builder

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"google.golang.org/grpc/credentials"

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
	"github.com/onflow/flow-go/engine/access/rest"
	restapiproxy "github.com/onflow/flow-go/engine/access/rest/apiproxy"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	rpcConnection "github.com/onflow/flow-go/engine/access/rpc/connection"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/common/follower"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/protocol"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	consensus_follower "github.com/onflow/flow-go/module/upstream"
	"github.com/onflow/flow-go/network"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/converter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/conduit"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/network/p2p/p2pnet"
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
	apiTimeout                time.Duration
	upstreamNodeAddresses     []string
	upstreamNodePublicKeys    []string
	upstreamIdentities        flow.IdentityList // the identity list of upstream peers the node uses to forward API requests to
}

// DefaultObserverServiceConfig defines all the default values for the ObserverServiceConfig
func DefaultObserverServiceConfig() *ObserverServiceConfig {
	return &ObserverServiceConfig{
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
		rpcMetricsEnabled:         false,
		apiRatelimits:             nil,
		apiBurstlimits:            nil,
		bootstrapNodeAddresses:    []string{},
		bootstrapNodePublicKeys:   []string{},
		observerNetworkingKeyPath: cmd.NotSet,
		apiTimeout:                3 * time.Second,
		upstreamNodeAddresses:     []string{},
		upstreamNodePublicKeys:    []string{},
	}
}

// ObserverServiceBuilder provides the common functionality needed to bootstrap a Flow observer service
// It is composed of the FlowNodeBuilder, the ObserverServiceConfig and contains all the components and modules needed for the observers
type ObserverServiceBuilder struct {
	*cmd.FlowNodeBuilder
	*ObserverServiceConfig

	// components
	LibP2PNode          p2p.LibP2PNode
	FollowerState       stateprotocol.FollowerState
	SyncCore            *chainsync.Core
	RpcEng              *rpc.Engine
	FollowerDistributor *pubsub.FollowerDistributor
	Committee           hotstuff.DynamicCommittee
	Finalized           *flow.Header
	Pending             []*flow.Header
	FollowerCore        module.HotStuffFollower

	// available until after the network has started. Hence, a factory function that needs to be called just before
	// creating the sync engine
	SyncEngineParticipantsProviderFactory func() module.IdentifierProvider

	// engines
	FollowerEng *follower.ComplianceEngine
	SyncEng     *synceng.Engine

	// Public network
	peerID peer.ID

	RestMetrics   *metrics.RestCollector
	AccessMetrics module.AccessMetrics
	// grpc servers
	secureGrpcServer   *grpcserver.GrpcServer
	unsecureGrpcServer *grpcserver.GrpcServer
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
			builder.ComplianceConfig,
			follower.WithChannel(channels.PublicReceiveBlocks),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}
		builder.FollowerDistributor.AddOnBlockFinalizedConsumer(builder.FollowerEng.OnFinalizedBlock)

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

		flags.StringVarP(&builder.rpcConf.UnsecureGRPCListenAddr, "rpc-addr", "r", defaultConfig.rpcConf.UnsecureGRPCListenAddr, "the address the unsecured gRPC server listens on")
		flags.StringVar(&builder.rpcConf.SecureGRPCListenAddr, "secure-rpc-addr", defaultConfig.rpcConf.SecureGRPCListenAddr, "the address the secure gRPC server listens on")
		flags.StringVarP(&builder.rpcConf.HTTPListenAddr, "http-addr", "h", defaultConfig.rpcConf.HTTPListenAddr, "the address the http proxy server listens on")
		flags.StringVar(&builder.rpcConf.RestConfig.ListenAddress, "rest-addr", defaultConfig.rpcConf.RestConfig.ListenAddress, "the address the REST server listens on (if empty the REST server will not be started)")
		flags.DurationVar(&builder.rpcConf.RestConfig.WriteTimeout, "rest-write-timeout", defaultConfig.rpcConf.RestConfig.WriteTimeout, "timeout to use when writing REST response")
		flags.DurationVar(&builder.rpcConf.RestConfig.ReadTimeout, "rest-read-timeout", defaultConfig.rpcConf.RestConfig.ReadTimeout, "timeout to use when reading REST request headers")
		flags.DurationVar(&builder.rpcConf.RestConfig.IdleTimeout, "rest-idle-timeout", defaultConfig.rpcConf.RestConfig.IdleTimeout, "idle timeout for REST connections")
		flags.UintVar(&builder.rpcConf.MaxMsgSize, "rpc-max-message-size", defaultConfig.rpcConf.MaxMsgSize, "the maximum message size in bytes for messages sent or received over grpc")
		flags.UintVar(&builder.rpcConf.BackendConfig.ConnectionPoolSize, "connection-pool-size", defaultConfig.rpcConf.BackendConfig.ConnectionPoolSize, "maximum number of connections allowed in the connection pool, size of 0 disables the connection pooling, and anything less than the default size will be overridden to use the default size")
		flags.UintVar(&builder.rpcConf.BackendConfig.MaxHeightRange, "rpc-max-height-range", defaultConfig.rpcConf.BackendConfig.MaxHeightRange, "maximum size for height range requests")
		flags.StringToIntVar(&builder.apiRatelimits, "api-rate-limits", defaultConfig.apiRatelimits, "per second rate limits for Access API methods e.g. Ping=300,GetTransaction=500 etc.")
		flags.StringToIntVar(&builder.apiBurstlimits, "api-burst-limits", defaultConfig.apiBurstlimits, "burst limits for Access API methods e.g. Ping=100,GetTransaction=100 etc.")
		flags.StringVar(&builder.observerNetworkingKeyPath, "observer-networking-key-path", defaultConfig.observerNetworkingKeyPath, "path to the networking key for observer")
		flags.StringSliceVar(&builder.bootstrapNodeAddresses, "bootstrap-node-addresses", defaultConfig.bootstrapNodeAddresses, "the network addresses of the bootstrap access node if this is an observer e.g. access-001.mainnet.flow.org:9653,access-002.mainnet.flow.org:9653")
		flags.StringSliceVar(&builder.bootstrapNodePublicKeys, "bootstrap-node-public-keys", defaultConfig.bootstrapNodePublicKeys, "the networking public key of the bootstrap access node if this is an observer (in the same order as the bootstrap node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")
		flags.DurationVar(&builder.apiTimeout, "upstream-api-timeout", defaultConfig.apiTimeout, "tcp timeout for Flow API gRPC sockets to upstrem nodes")
		flags.StringSliceVar(&builder.upstreamNodeAddresses, "upstream-node-addresses", defaultConfig.upstreamNodeAddresses, "the gRPC network addresses of the upstream access node. e.g. access-001.mainnet.flow.org:9000,access-002.mainnet.flow.org:9000")
		flags.StringSliceVar(&builder.upstreamNodePublicKeys, "upstream-node-public-keys", defaultConfig.upstreamNodePublicKeys, "the networking public key of the upstream access node (in the same order as the upstream node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")
		flags.BoolVar(&builder.rpcMetricsEnabled, "rpc-metrics-enabled", defaultConfig.rpcMetricsEnabled, "whether to enable the rpc metrics")
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

		// The following wrapper allows to black-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of disallow-listed nodes to true
		builder.IdentityProvider, err = cache.NewNodeDisallowListWrapper(idCache, node.DB, func() network.DisallowListNotificationConsumer {
			return builder.NetworkUnderlay
		})
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
						builder.Logger.Err(err).Str("peer", p2plogging.PeerId(pid)).Msg("failed to translate to Flow ID")
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

// initPublicLibp2pNode creates a libp2p node for the observer service in the public (unstaked) network.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
// The LibP2P host is created with the following options:
// * DHT as client and seeded with the given bootstrap peers
// * The specified bind address as the listen address
// * The passed in private key as the libp2p key
// * No connection gater
// * No connection manager
// * No peer manager
// * Default libp2p pubsub options.
// Args:
// - networkKey: the private key to use for the libp2p node
// Returns:
// - p2p.LibP2PNode: the libp2p node
// - error: if any error occurs. Any error returned is considered irrecoverable.
func (builder *ObserverServiceBuilder) initPublicLibp2pNode(networkKey crypto.PrivateKey) (p2p.LibP2PNode, error) {
	var pis []peer.AddrInfo

	for _, b := range builder.bootstrapIdentities {
		pi, err := utils.PeerAddressInfo(*b)
		if err != nil {
			return nil, fmt.Errorf("could not extract peer address info from bootstrap identity %v: %w", b, err)
		}

		pis = append(pis, pi)
	}

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             builder.Logger,
		Metrics:                            builder.Metrics.Network,
		IDProvider:                         builder.IdentityProvider,
		LoggerInterval:                     builder.FlowConfig.NetworkConfig.GossipSubConfig.LocalMeshLogInterval,
		RpcSentTrackerCacheSize:            builder.FlowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: builder.FlowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         builder.FlowConfig.NetworkConfig.GossipSubConfig.RpcSentTrackerNumOfWorkers,
		HeroCacheMetricsFactory:            builder.HeroCacheMetricsFactory(),
		NetworkingType:                     network.PublicNetwork,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)

	node, err := p2pbuilder.NewNodeBuilder(
		builder.Logger,
		&p2pconfig.MetricsConfig{
			HeroCacheFactory: builder.HeroCacheMetricsFactory(),
			Metrics:          builder.Metrics.Network,
		},
		network.PublicNetwork,
		builder.BaseConfig.BindAddr,
		networkKey,
		builder.SporkID,
		builder.IdentityProvider,
		&builder.FlowConfig.NetworkConfig.ResourceManager,
		&builder.FlowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig,
		p2pconfig.PeerManagerDisableConfig(), // disable peer manager for observer node.
		&builder.FlowConfig.NetworkConfig.GossipSubConfig.SubscriptionProviderConfig,
		&p2p.DisallowListCacheConfig{
			MaxSize: builder.FlowConfig.NetworkConfig.DisallowListNotificationCacheSize,
			Metrics: metrics.DisallowListCacheMetricsFactory(builder.HeroCacheMetricsFactory(), network.PublicNetwork),
		},
		meshTracer,
		&p2pconfig.UnicastConfig{
			UnicastConfig: builder.FlowConfig.NetworkConfig.UnicastConfig,
		}).
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
		SetGossipSubTracer(meshTracer).
		SetGossipSubScoreTracerInterval(builder.FlowConfig.NetworkConfig.GossipSubConfig.ScoreTracerInterval).
		Build()

	if err != nil {
		return nil, fmt.Errorf("could not initialize libp2p node for observer: %w", err)
	}

	builder.LibP2PNode = node

	return builder.LibP2PNode, nil
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
	return builder.FlowNodeBuilder.Build()
}

// enqueuePublicNetworkInit enqueues the observer network component initialized for the observer
func (builder *ObserverServiceBuilder) enqueuePublicNetworkInit() {
	var publicLibp2pNode p2p.LibP2PNode
	builder.
		Component("public libp2p node", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			var err error
			publicLibp2pNode, err = builder.initPublicLibp2pNode(node.NetworkKey)
			if err != nil {
				return nil, fmt.Errorf("could not create public libp2p node: %w", err)
			}

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

			net, err := p2pnet.NewNetwork(&p2pnet.NetworkConfig{
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
				UnicastMessageTimeout: p2pnet.DefaultUnicastTimeout,
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
			}, p2pnet.WithMessageValidators(publicNetworkMsgValidators(node.Logger, node.IdentityProvider, node.NodeID)...))
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
	builder.Module("creating grpc servers", func(node *cmd.NodeConfig) error {
		builder.secureGrpcServer = grpcserver.NewGrpcServerBuilder(node.Logger,
			builder.rpcConf.SecureGRPCListenAddr,
			builder.rpcConf.MaxMsgSize,
			builder.rpcMetricsEnabled,
			builder.apiRatelimits,
			builder.apiBurstlimits,
			grpcserver.WithTransportCredentials(builder.rpcConf.TransportCredentials)).Build()

		builder.unsecureGrpcServer = grpcserver.NewGrpcServerBuilder(node.Logger,
			builder.rpcConf.UnsecureGRPCListenAddr,
			builder.rpcConf.MaxMsgSize,
			builder.rpcMetricsEnabled,
			builder.apiRatelimits,
			builder.apiBurstlimits).Build()

		return nil
	})
	builder.Module("rest metrics", func(node *cmd.NodeConfig) error {
		m, err := metrics.NewRestCollector(routes.URLToRoute, node.MetricsRegisterer)
		if err != nil {
			return err
		}
		builder.RestMetrics = m
		return nil
	})
	builder.Module("access metrics", func(node *cmd.NodeConfig) error {
		builder.AccessMetrics = metrics.NewAccessCollector(
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
	builder.Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		accessMetrics := builder.AccessMetrics
		config := builder.rpcConf
		backendConfig := config.BackendConfig
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
			CollectionGRPCPort:        0,
			ExecutionGRPCPort:         0,
			CollectionNodeGRPCTimeout: builder.apiTimeout,
			ExecutionNodeGRPCTimeout:  builder.apiTimeout,
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

		accessBackend, err := backend.New(backend.Params{
			State:                     node.State,
			Blocks:                    node.Storage.Blocks,
			Headers:                   node.Storage.Headers,
			Collections:               node.Storage.Collections,
			Transactions:              node.Storage.Transactions,
			ExecutionReceipts:         node.Storage.Receipts,
			ExecutionResults:          node.Storage.Results,
			ChainID:                   node.RootChainID,
			AccessMetrics:             accessMetrics,
			ConnFactory:               connFactory,
			RetryEnabled:              false,
			MaxHeightRange:            backendConfig.MaxHeightRange,
			PreferredExecutionNodeIDs: backendConfig.PreferredExecutionNodeIDs,
			FixedExecutionNodeIDs:     backendConfig.FixedExecutionNodeIDs,
			Log:                       node.Logger,
			SnapshotHistoryLimit:      backend.DefaultSnapshotHistoryLimit,
			Communicator:              backend.NewNodeCommunicator(backendConfig.CircuitBreakerConfig.Enabled),
		})
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

		stateStreamConfig := statestreambackend.Config{}
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
			nil, // state streaming is not supported
			stateStreamConfig,
		)
		if err != nil {
			return nil, err
		}

		// upstream access node forwarder
		forwarder, err := apiproxy.NewFlowAccessAPIForwarder(builder.upstreamIdentities, connFactory)
		if err != nil {
			return nil, err
		}

		rpcHandler := &apiproxy.FlowAccessAPIRouter{
			Logger:   builder.Logger,
			Metrics:  observerCollector,
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
			WithRpcHandler(rpcHandler).
			WithLegacy().
			Build()
		if err != nil {
			return nil, err
		}
		builder.FollowerDistributor.AddOnBlockFinalizedConsumer(builder.RpcEng.OnFinalizedBlock)
		return builder.RpcEng, nil
	})

	// build secure grpc server
	builder.Component("secure grpc server", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return builder.secureGrpcServer, nil
	})

	// build unsecure grpc server
	builder.Component("unsecure grpc server", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return builder.unsecureGrpcServer, nil
	})
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
