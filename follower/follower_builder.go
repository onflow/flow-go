package follower

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/common/follower"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	synchronization "github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/compliance"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/upstream"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/converter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/cache"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	storage "github.com/onflow/flow-go/storage/badger"
)

// FlowBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Access node
// These functions are shared by staked and observer builders.
// The Staked network allows the staked nodes to communicate among themselves, while the public network allows the
// observers and an Access node to communicate.
//
//                                 public network                           staked network
//  +------------------------+
//  | observer 1             |<--------------------------|
//  +------------------------+                           v
//  +------------------------+                         +----------------------+              +------------------------+
//  | observer 2             |<----------------------->| Access Node (staked) |<------------>| All other staked Nodes |
//  +------------------------+                         +----------------------+              +------------------------+
//  +------------------------+                           ^
//  | observer 3             |<--------------------------|
//  +------------------------+

// FollowerServiceConfig defines all the user defined parameters required to bootstrap an access node
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type FollowerServiceConfig struct {
	bootstrapNodeAddresses  []string
	bootstrapNodePublicKeys []string
	bootstrapIdentities     flow.IdentityList // the identity list of bootstrap peers the node uses to discover other nodes
	NetworkKey              crypto.PrivateKey // the networking key passed in by the caller when being used as a library
	baseOptions             []cmd.Option
}

// DefaultFollowerServiceConfig defines all the default values for the FollowerServiceConfig
func DefaultFollowerServiceConfig() *FollowerServiceConfig {
	return &FollowerServiceConfig{
		bootstrapNodeAddresses:  []string{},
		bootstrapNodePublicKeys: []string{},
	}
}

// FollowerServiceBuilder provides the common functionality needed to bootstrap a Flow staked and observer
// It is composed of the FlowNodeBuilder, the FollowerServiceConfig and contains all the components and modules needed for the
// staked and observers
type FollowerServiceBuilder struct {
	*cmd.FlowNodeBuilder
	*FollowerServiceConfig

	// components
	LibP2PNode              p2p.LibP2PNode
	FollowerState           protocol.MutableState
	SyncCore                *synchronization.Core
	FinalizationDistributor *pubsub.FinalizationDistributor
	FinalizedHeader         *synceng.FinalizedHeaderCache
	Committee               hotstuff.Committee
	Finalized               *flow.Header
	Pending                 []*flow.Header
	FollowerCore            module.HotStuffFollower
	// for the observer, the sync engine participants provider is the libp2p peer store which is not
	// available until after the network has started. Hence, a factory function that needs to be called just before
	// creating the sync engine
	SyncEngineParticipantsProviderFactory func() module.IdentifierProvider

	// engines
	FollowerEng *followereng.Engine
	SyncEng     *synceng.Engine

	peerID peer.ID
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstrap peers from the parameters.
// These are the identities of the staked and observers also acting as the DHT bootstrap server
func (builder *FollowerServiceBuilder) deriveBootstrapPeerIdentities() error {
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

func (builder *FollowerServiceBuilder) buildFollowerState() *FollowerServiceBuilder {
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

func (builder *FollowerServiceBuilder) buildSyncCore() *FollowerServiceBuilder {
	builder.Module("sync core", func(node *cmd.NodeConfig) error {
		syncCore, err := synchronization.New(node.Logger, node.SyncCoreConfig, metrics.NewChainSyncCollector())
		builder.SyncCore = syncCore

		return err
	})

	return builder
}

func (builder *FollowerServiceBuilder) buildCommittee() *FollowerServiceBuilder {
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

func (builder *FollowerServiceBuilder) buildLatestHeader() *FollowerServiceBuilder {
	builder.Module("latest header", func(node *cmd.NodeConfig) error {
		finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
		builder.Finalized, builder.Pending = finalized, pending

		return err
	})

	return builder
}

func (builder *FollowerServiceBuilder) buildFollowerCore() *FollowerServiceBuilder {
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

func (builder *FollowerServiceBuilder) buildFollowerEngine() *FollowerServiceBuilder {
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
			follower.WithComplianceOptions(compliance.WithSkipNewProposalsThreshold(builder.ComplianceConfig.SkipNewProposalsThreshold)),
			follower.WithChannel(channels.PublicReceiveBlocks),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}
		builder.FollowerEng = followerEng

		return builder.FollowerEng, nil
	})

	return builder
}

func (builder *FollowerServiceBuilder) buildFinalizedHeader() *FollowerServiceBuilder {
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

func (builder *FollowerServiceBuilder) buildSyncEngine() *FollowerServiceBuilder {
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

func (builder *FollowerServiceBuilder) BuildConsensusFollower() cmd.NodeBuilder {
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

type FollowerOption func(*FollowerServiceConfig)

func WithBootStrapPeers(bootstrapNodes ...*flow.Identity) FollowerOption {
	return func(config *FollowerServiceConfig) {
		config.bootstrapIdentities = bootstrapNodes
	}
}

func WithNetworkKey(key crypto.PrivateKey) FollowerOption {
	return func(config *FollowerServiceConfig) {
		config.NetworkKey = key
	}
}

func WithBaseOptions(baseOptions []cmd.Option) FollowerOption {
	return func(config *FollowerServiceConfig) {
		config.baseOptions = baseOptions
	}
}

func FlowConsensusFollowerService(opts ...FollowerOption) *FollowerServiceBuilder {
	config := DefaultFollowerServiceConfig()
	for _, opt := range opts {
		opt(config)
	}
	ret := &FollowerServiceBuilder{
		FollowerServiceConfig: config,
		// TODO: using RoleAccess here for now. This should be refactored eventually to have its own role type
		FlowNodeBuilder:         cmd.FlowNode(flow.RoleAccess.String(), config.baseOptions...),
		FinalizationDistributor: pubsub.NewFinalizationDistributor(),
	}
	// the observer gets a version of the root snapshot file that does not contain any node addresses
	// hence skip all the root snapshot validations that involved an identity address
	ret.FlowNodeBuilder.SkipNwAddressBasedValidations = true
	return ret
}

// initNetwork creates the network.Network implementation with the given metrics, middleware, initial list of network
// participants and topology used to choose peers from the list of participants. The list of participants can later be
// updated by calling network.SetIDs.
func (builder *FollowerServiceBuilder) initNetwork(nodeID module.Local,
	networkMetrics module.NetworkMetrics,
	middleware network.Middleware,
	topology network.Topology,
	receiveCache *netcache.ReceiveCache,
) (*p2p.Network, error) {

	codec := cborcodec.NewCodec()

	// creates network instance
	net, err := p2p.NewNetwork(&p2p.NetworkParameters{
		Logger:              builder.Logger,
		Codec:               codec,
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
			NodeID:        flow.ZeroID, // the NodeID is the hash of the staking key and for the public network it does not apply
			Address:       address,
			Role:          flow.RoleAccess, // the upstream node has to be an access node
			NetworkPubKey: networkKey,
		}
	}
	return ids, nil
}

func (builder *FollowerServiceBuilder) initNodeInfo() error {
	// use the networking key that has been passed in the config, or load from the configured file
	networkingKey := builder.FollowerServiceConfig.NetworkKey

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

func (builder *FollowerServiceBuilder) InitIDProviders() {
	builder.Module("id providers", func(node *cmd.NodeConfig) error {
		idCache, err := cache.NewProtocolStateIDCache(node.Logger, node.State, builder.ProtocolEvents)
		if err != nil {
			return fmt.Errorf("could not initialize ProtocolStateIDCache: %w", err)
		}
		builder.IDTranslator = translator.NewHierarchicalIDTranslator(idCache, translator.NewPublicNetworkIDTranslator())

		// The following wrapper allows to black-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of blocked nodes to true
		builder.NodeBlockListDistributor = cache.NewNodeBlockListDistributor()
		builder.IdentityProvider, err = cache.NewNodeBlocklistWrapper(idCache, node.DB, builder.NodeBlockListDistributor)
		if err != nil {
			return fmt.Errorf("could not initialize NodeBlocklistWrapper: %w", err)
		}

		// use the default identifier provider
		builder.SyncEngineParticipantsProviderFactory = func() module.IdentifierProvider {
			return id.NewCustomIdentifierProvider(func() flow.IdentifierList {
				pids := builder.LibP2PNode.GetPeersForProtocol(unicast.FlowProtocolID(builder.SporkID))
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
}

func (builder *FollowerServiceBuilder) Initialize() error {
	if err := builder.deriveBootstrapPeerIdentities(); err != nil {
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

func (builder *FollowerServiceBuilder) validateParams() error {
	if builder.BaseConfig.BindAddr == cmd.NotSet || builder.BaseConfig.BindAddr == "" {
		return errors.New("bind address not specified")
	}
	if builder.FollowerServiceConfig.NetworkKey == nil {
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
	return nil
}

// initLibP2PFactory creates the LibP2P factory function for the given node ID and network key for the observer.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
// The LibP2P host is created with the following options:
//   - DHT as client and seeded with the given bootstrap peers
//   - The specified bind address as the listen address
//   - The passed in private key as the libp2p key
//   - No connection gater
//   - No connection manager
//   - No peer manager
//   - Default libp2p pubsub options
func (builder *FollowerServiceBuilder) initLibP2PFactory(networkKey crypto.PrivateKey) p2pbuilder.LibP2PFactoryFunc {
	return func() (p2p.LibP2PNode, error) {
		var pis []peer.AddrInfo

		for _, b := range builder.bootstrapIdentities {
			pi, err := utils.PeerAddressInfo(*b)

			if err != nil {
				return nil, fmt.Errorf("could not extract peer address info from bootstrap identity %v: %w", b, err)
			}

			pis = append(pis, pi)
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
				return p2pdht.NewDHT(ctx, h, unicast.FlowPublicDHTProtocolID(builder.SporkID),
					builder.Logger,
					builder.Metrics.Network,
					p2pdht.AsClient(),
					dht.BootstrapPeers(pis...),
				)
			}).
			Build()

		if err != nil {
			return nil, fmt.Errorf("could not build libp2p node: %w", err)
		}

		builder.LibP2PNode = node

		return builder.LibP2PNode, nil
	}
}

// initObserverLocal initializes the observer's ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (builder *FollowerServiceBuilder) initObserverLocal() func(node *cmd.NodeConfig) error {
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
func (builder *FollowerServiceBuilder) Build() (cmd.Node, error) {
	builder.BuildConsensusFollower()
	return builder.FlowNodeBuilder.Build()
}

// enqueuePublicNetworkInit enqueues the observer network component initialized for the observer
func (builder *FollowerServiceBuilder) enqueuePublicNetworkInit() {
	var libp2pNode p2p.LibP2PNode
	builder.
		Component("public libp2p node", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			libP2PFactory := builder.initLibP2PFactory(node.NetworkKey)

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
// AN with the staked AN.
// Currently, there is an issue with LibP2P stopping advertisements of subscribed topics if no peers are connected
// (https://github.com/libp2p/go-libp2p-pubsub/issues/442). This means that an observer could end up not being
// discovered by other observers if it subscribes to a topic before connecting to the staked AN. Hence, the need
// of an explicit connect to the staked AN before the node attempts to subscribe to topics.
func (builder *FollowerServiceBuilder) enqueueConnectWithStakedAN() {
	builder.Component("upstream connector", func(_ *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return upstream.NewUpstreamConnector(builder.bootstrapIdentities, builder.LibP2PNode, builder.Logger), nil
	})
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (builder *FollowerServiceBuilder) initMiddleware(nodeID flow.Identifier,
	libp2pNode p2p.LibP2PNode,
	validators ...network.MessageValidator,
) network.Middleware {
	slashingViolationsConsumer := slashing.NewSlashingViolationsConsumer(builder.Logger, builder.Metrics.Network)
	mw := middleware.NewMiddleware(
		builder.Logger,
		libp2pNode,
		nodeID,
		builder.Metrics.Bitswap,
		builder.SporkID,
		middleware.DefaultUnicastTimeout,
		builder.IDTranslator,
		builder.CodecFactory(),
		slashingViolationsConsumer,
		middleware.WithMessageValidators(validators...),
		// use default identifier provider
	)
	builder.NodeBlockListDistributor.AddConsumer(mw)
	builder.Middleware = mw
	return builder.Middleware
}
