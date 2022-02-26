package node_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/crypto"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine/access/ingestion"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/follower"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
)

// AccessNodeBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Access node
// These functions are shared by staked and unstaked access node builders.
// The Staked network allows the staked nodes to communicate among themselves, while the unstaked network allows the
// unstaked nodes and a staked Access node to communicate.
//
//                                 unstaked network                           staked network
//  +------------------------+
//  | Unstaked Access Node 1 |<--------------------------|
//  +------------------------+                           v
//  +------------------------+                         +--------------------+                 +------------------------+
//  | Unstaked Access Node 2 |<----------------------->| Staked Access Node |<--------------->| All other staked Nodes |
//  +------------------------+                         +--------------------+                 +------------------------+
//  +------------------------+                           ^
//  | Unstaked Access Node 3 |<--------------------------|
//  +------------------------+

type AccessNodeBuilder interface {
	cmd.NodeBuilder

	// IsStaked returns True if this is a staked Access Node, False otherwise
	IsStaked() bool
}

// AccessNodeConfig defines all the user defined parameters required to bootstrap an access node
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type AccessNodeConfig struct {
	staked                       bool
	bootstrapNodeAddresses       []string
	bootstrapNodePublicKeys      []string
	observerNetworkingKeyPath    string
	bootstrapIdentities          flow.IdentityList // the identity list of bootstrap peers the node uses to discover other nodes
	NetworkKey                   crypto.PrivateKey // the networking key passed in by the caller when being used as a library
	supportsUnstakedFollower     bool              // True if this is a staked Access node which also supports unstaked access nodes/unstaked consensus follower engines
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
	return &AccessNodeConfig{
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
		staked:                       true,
		bootstrapNodeAddresses:       []string{},
		bootstrapNodePublicKeys:      []string{},
		supportsUnstakedFollower:     false,
		PublicNetworkConfig: PublicNetworkConfig{
			BindAddress: cmd.NotSet,
			Metrics:     metrics.NewNoopCollector(),
		},
		observerNetworkingKeyPath: cmd.NotSet,
	}
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow staked and unstaked access node
// It is composed of the FlowNodeBuilder, the AccessNodeConfig and contains all the components and modules needed for the
// staked and unstaked access nodes
type FlowAccessNodeBuilder struct {
	*cmd.FlowNodeBuilder
	*AccessNodeConfig

	// components
	LibP2PNode                 *p2p.Node
	FollowerState              protocol.MutableState
	SyncCore                   *synchronization.Core
	RpcEng                     *rpc.Engine
	FinalizationDistributor    *pubsub.FinalizationDistributor
	FinalizedHeader            *synceng.FinalizedHeaderCache
	CollectionRPC              access.AccessAPIClient
	TransactionTimings         *stdmap.TransactionTimings
	CollectionsToMarkFinalized *stdmap.Times
	CollectionsToMarkExecuted  *stdmap.Times
	BlocksToMarkExecuted       *stdmap.Times
	TransactionMetrics         module.TransactionMetrics
	PingMetrics                module.PingMetrics
	Committee                  hotstuff.Committee
	Finalized                  *flow.Header
	Pending                    []*flow.Header
	FollowerCore               module.HotStuffFollower
	// for the unstaked access node, the sync engine participants provider is the libp2p peer store which is not
	// available until after the network has started. Hence, a factory function that needs to be called just before
	// creating the sync engine
	SyncEngineParticipantsProviderFactory func() id.IdentifierProvider

	// engines
	IngestEng   *ingestion.Engine
	RequestEng  *requester.Engine
	FollowerEng *followereng.Engine
	SyncEng     *synceng.Engine
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstrap peers from the parameters.
// These are the identities of the staked and unstaked ANs also acting as the DHT bootstrap server
func (builder *FlowAccessNodeBuilder) deriveBootstrapPeerIdentities() error {
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
		syncCore, err := synchronization.New(node.Logger, synchronization.DefaultConfig())
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

		packer := hotsignature.NewConsensusSigDataPacker(builder.Committee)
		// initialize the verifier for the protocol consensus
		verifier := verification.NewCombinedVerifier(builder.Committee, packer)

		followerCore, err := consensus.NewFollower(node.Logger, builder.Committee, node.Storage.Headers, final, verifier,
			builder.FinalizationDistributor, node.RootBlock.Header, node.RootQC, builder.Finalized, builder.Pending)
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
		cleaner := storage.NewCleaner(node.Logger, node.DB, builder.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)
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

func (builder *FlowAccessNodeBuilder) BuildConsensusFollower() AccessNodeBuilder {
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

type Option func(*AccessNodeConfig)

func WithBootStrapPeers(bootstrapNodes ...*flow.Identity) Option {
	return func(config *AccessNodeConfig) {
		config.bootstrapIdentities = bootstrapNodes
	}
}

func SupportsUnstakedNode(enable bool) Option {
	return func(config *AccessNodeConfig) {
		config.supportsUnstakedFollower = enable
	}
}

func WithNetworkKey(key crypto.PrivateKey) Option {
	return func(config *AccessNodeConfig) {
		config.NetworkKey = key
	}
}

func WithBaseOptions(baseOptions []cmd.Option) Option {
	return func(config *AccessNodeConfig) {
		config.baseOptions = baseOptions
	}
}

func FlowAccessNode(opts ...Option) *FlowAccessNodeBuilder {
	config := DefaultAccessNodeConfig()
	for _, opt := range opts {
		opt(config)
	}

	return &FlowAccessNodeBuilder{
		AccessNodeConfig:        config,
		FlowNodeBuilder:         cmd.FlowNode(flow.RoleAccess.String(), config.baseOptions...),
		FinalizationDistributor: pubsub.NewFinalizationDistributor(),
	}
}
func (builder *FlowAccessNodeBuilder) IsStaked() bool {
	return builder.staked
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
		flags.BoolVar(&builder.staked, "staked", defaultConfig.staked, "whether this node is a staked access node or not")
		flags.StringVar(&builder.observerNetworkingKeyPath, "observer-networking-key-path", defaultConfig.observerNetworkingKeyPath, "path to the networking key for observer")
		flags.StringSliceVar(&builder.bootstrapNodeAddresses, "bootstrap-node-addresses", defaultConfig.bootstrapNodeAddresses, "the network addresses of the bootstrap access node if this is an unstaked access node e.g. access-001.mainnet.flow.org:9653,access-002.mainnet.flow.org:9653")
		flags.StringSliceVar(&builder.bootstrapNodePublicKeys, "bootstrap-node-public-keys", defaultConfig.bootstrapNodePublicKeys, "the networking public key of the bootstrap access node if this is an unstaked access node (in the same order as the bootstrap node addresses) e.g. \"d57a5e9c5.....\",\"44ded42d....\"")
		flags.BoolVar(&builder.supportsUnstakedFollower, "supports-unstaked-node", defaultConfig.supportsUnstakedFollower, "true if this staked access node supports unstaked node")
		flags.StringVar(&builder.PublicNetworkConfig.BindAddress, "public-network-address", defaultConfig.PublicNetworkConfig.BindAddress, "staked access node's public network bind address")
	}).ValidateFlags(func() error {
		if builder.supportsUnstakedFollower && (builder.PublicNetworkConfig.BindAddress == cmd.NotSet || builder.PublicNetworkConfig.BindAddress == "") {
			return errors.New("public-network-address must be set if supports-unstaked-node is true")
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
) (*p2p.Network, error) {

	codec := cborcodec.NewCodec()

	// creates network instance
	net, err := p2p.NewNetwork(
		builder.Logger,
		codec,
		nodeID,
		func() (network.Middleware, error) { return builder.Middleware, nil },
		p2p.DefaultCacheSize,
		topology,
		p2p.NewChannelSubscriptionManager(middleware),
		networkMetrics,
		builder.IdentityProvider,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize network: %w", err)
	}

	return net, nil
}

func unstakedNetworkMsgValidators(log zerolog.Logger, idProvider id.IdentityProvider, selfID flow.Identifier) []network.MessageValidator {
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
		key := keys[i]

		// json unmarshaller needs a quotes before and after the string
		// the pflags.StringSliceVar does not retain quotes for the command line arg even if escaped with \"
		// hence this additional check to ensure the key is indeed quoted
		if !strings.HasPrefix(key, "\"") {
			key = fmt.Sprintf("\"%s\"", key)
		}
		// networking public key
		var networkKey encodable.NetworkPubKey
		err := json.Unmarshal([]byte(key), &networkKey)
		if err != nil {
			return nil, err
		}

		// create the identity of the peer by setting only the relevant fields
		ids[i] = &flow.Identity{
			NodeID:        flow.ZeroID, // the NodeID is the hash of the staking key and for the unstaked network it does not apply
			Address:       address,
			Role:          flow.RoleAccess, // the upstream node has to be an access node
			NetworkPubKey: networkKey,
		}
	}
	return ids, nil
}
