package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	storage "github.com/onflow/flow-go/storage/badger"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
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

	Build()

	// IsStaked returns True is this is a staked Access Node, False otherwise
	IsStaked() bool

	// ParticipatesInUnstakedNetwork returns True if this is a staked Access node which also participates
	// in the unstaked network acting as an upstream for other unstaked access nodes, False otherwise.
	ParticipatesInUnstakedNetwork() bool
}

type AccessNodeConfig struct {
	stakedAccessNodeIDHex        string
	unstakedNetworkBindAddr      string
	blockLimit                   uint
	collectionLimit              uint
	receiptLimit                 uint
	collectionGRPCPort           uint
	executionGRPCPort            uint
	pingEnabled                  bool
	nodeInfoFile                 string
	apiRatelimits                map[string]int
	apiBurstlimits               map[string]int
	followerState                protocol.MutableState
	ingestEng                    *ingestion.Engine
	requestEng                   *requester.Engine
	followerEng                  *followereng.Engine
	syncCore                     *synchronization.Core
	rpcConf                      rpc.Config
	rpcEng                       *rpc.Engine
	finalizationDistributor      *pubsub.FinalizationDistributor
	collectionRPC                access.AccessAPIClient
	executionNodeAddress         string // deprecated
	historicalAccessRPCs         []access.AccessAPIClient
	err                          error
	conCache                     *buffer.PendingBlocks // pending block cache for follower
	transactionTimings           *stdmap.TransactionTimings
	collectionsToMarkFinalized   *stdmap.Times
	collectionsToMarkExecuted    *stdmap.Times
	blocksToMarkExecuted         *stdmap.Times
	transactionMetrics           module.TransactionMetrics
	pingMetrics                  module.PingMetrics
	logTxTimeToFinalized         bool
	logTxTimeToExecuted          bool
	logTxTimeToFinalizedExecuted bool
	retryEnabled                 bool
	rpcMetricsEnabled            bool
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow staked and unstaked access node
// It is composed of the FlowNodeBuilder
type FlowAccessNodeBuilder struct {
	*cmd.FlowNodeBuilder
	*AccessNodeConfig
	staked             bool
	UnstakedNetwork    *p2p.Network
	unstakedMiddleware *p2p.Middleware
	followerState      protocol.MutableState
}

func FlowAccessNode() *FlowAccessNodeBuilder {
	return &FlowAccessNodeBuilder{
		AccessNodeConfig: &AccessNodeConfig{},
		FlowNodeBuilder:  cmd.FlowNode(flow.RoleAccess.String()),
	}
}
func (builder *FlowAccessNodeBuilder) IsStaked() bool {
	return builder.staked
}

func (builder *FlowAccessNodeBuilder) ParticipatesInUnstakedNetwork() bool {
	// unstaked access nodes can't be upstream of other unstaked access nodes for now
	if !builder.IsStaked() {
		return false
	}
	// if an unstaked network bind address is provided, then this staked access node will act as the upstream for
	// unstaked access nodes
	return builder.unstakedNetworkBindAddr != cmd.NotSet
}

func (builder *FlowAccessNodeBuilder) parseFlags() {

	builder.BaseFlags()

	builder.ParseAndPrintFlags()
}

// initLibP2PFactory creates the LibP2P factory function for the given node ID and network key.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
func (builder *FlowAccessNodeBuilder) initLibP2PFactory(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	networkKey crypto.PrivateKey) (p2p.LibP2PFactoryFunc, error) {

	// setup the Ping provider to return the software version and the sealed block height
	pingProvider := p2p.PingInfoProviderImpl{
		SoftwareVersionFun: func() string {
			return build.Semver()
		},
		SealedBlockHeightFun: func() (uint64, error) {
			head, err := builder.State.Sealed().Head()
			if err != nil {
				return 0, err
			}
			return head.Height, nil
		},
	}

	libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(builder.Logger,
		nodeID,
		builder.unstakedNetworkBindAddr,
		networkKey,
		builder.RootBlock.ID().String(),
		p2p.DefaultMaxPubSubMsgSize,
		networkMetrics,
		pingProvider)
	if err != nil {
		return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
	}

	return libP2PNodeFactory, nil
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (builder *FlowAccessNodeBuilder) initMiddleware(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	factoryFunc p2p.LibP2PFactoryFunc,
	peerUpdateInterval time.Duration,
	connectionGating bool,
	managerPeerConnections bool,
	validators ...network.MessageValidator) *p2p.Middleware {
	builder.unstakedMiddleware = p2p.NewMiddleware(builder.Logger,
		factoryFunc,
		nodeID,
		networkMetrics,
		builder.RootBlock.ID().String(),
		peerUpdateInterval,
		p2p.DefaultUnicastTimeout,
		connectionGating,
		managerPeerConnections,
		validators...)
	return builder.unstakedMiddleware
}

// initNetwork creates the network.Network implementation with the given metrics, middleware, initial list of network
// participants and topology used to choose peers from the list of participants. The list of participants can later be
// updated by calling network.SetIDs.
func (builder *FlowAccessNodeBuilder) initNetwork(nodeID module.Local,
	networkMetrics module.NetworkMetrics,
	middleware *p2p.Middleware,
	participants flow.IdentityList,
	topology network.Topology) (*p2p.Network, error) {

	codec := jsoncodec.NewCodec()

	subscriptionManager := p2p.NewChannelSubscriptionManager(middleware)

	// creates network instance
	net, err := p2p.NewNetwork(builder.Logger,
		codec,
		participants,
		nodeID,
		builder.unstakedMiddleware,
		p2p.DefaultCacheSize,
		topology,
		subscriptionManager,
		networkMetrics)
	if err != nil {
		return nil, fmt.Errorf("could not initialize network: %w", err)
	}

	return net, nil
}

func (builder *FlowAccessNodeBuilder) mutableFollowerStateModule() *FlowAccessNodeBuilder {
	builder.Module("mutable follower state", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
		// For now, we only support state implementations from package badger.
		// If we ever support different implementations, the following can be replaced by a type-aware factory
		state, ok := node.State.(*badgerState.State)
		if !ok {
			return fmt.Errorf("only implementations of type badger.State are currenlty supported but read-only state has type %T", node.State)
		}
		followerState, err := badgerState.NewFollowerState(
			state,
			node.Storage.Index,
			node.Storage.Payloads,
			node.Tracer,
			node.ProtocolEvents,
			blocktimer.DefaultBlockTimer,
		)
		if err != nil {
			return err
		}
		builder.followerState = followerState
		return nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) collectionNodeClientModule() *FlowAccessNodeBuilder {
	builder.Module("collection node client", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
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
			grpc.WithInsecure(),
			backend.WithClientUnaryInterceptor(builder.rpcConf.CollectionClientTimeout))
		if err != nil {
			return err
		}
		builder.collectionRPC = access.NewAccessAPIClient(collectionRPCConn)
		return nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) historialAccessNodeClientModule() *FlowAccessNodeBuilder {
	builder.Module("historical access node clients", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
		addrs := strings.Split(builder.rpcConf.HistoricalAccessAddrs, ",")
		for _, addr := range addrs {
			if strings.TrimSpace(addr) == "" {
				continue
			}
			node.Logger.Info().Str("access_nodes", addr).Msg("historical access node addresses")

			historicalAccessRPCConn, err := grpc.Dial(
				addr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure())
			if err != nil {
				return err
			}
			builder.historicalAccessRPCs = append(builder.historicalAccessRPCs, access.NewAccessAPIClient(historicalAccessRPCConn))
		}
		return nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) followerEnginerDepsModule() *FlowAccessNodeBuilder {
	builder.Module("block cache", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
		builder.conCache = buffer.NewPendingBlocks()
		return nil
	}).
		Module("sync core", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
			var err error
			builder.syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		})
	return builder
}

func (builder *FlowAccessNodeBuilder) transactionTimingMempoolsModule() *FlowAccessNodeBuilder {
	builder.Module("transaction timing mempools", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
		var err error
		builder.transactionTimings, err = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
		if err != nil {
			return err
		}

		builder.collectionsToMarkFinalized, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
		if err != nil {
			return err
		}

		builder.collectionsToMarkExecuted, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
		if err != nil {
			return err
		}

		builder.blocksToMarkExecuted, err = stdmap.NewTimes(1 * 300) // assume 1 block per second * 300 seconds
		return err
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) pingMetricsModule() *FlowAccessNodeBuilder {
	builder.Module("ping metrics", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
		builder.pingMetrics = metrics.NewPingCollector()
		return nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) grpcCertificateModule() *FlowAccessNodeBuilder {
	builder.Module("server certificate", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {
		// generate the server certificate that will be served by the GRPC server
		x509Certificate, err := grpcutils.X509Certificate(node.NetworkKey)
		if err != nil {
			return err
		}
		tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
		builder.rpcConf.TransportCredentials = credentials.NewTLS(tlsConfig)
		return nil
	})
	return builder
}
func (builder *FlowAccessNodeBuilder) rpcEngineComponent() *FlowAccessNodeBuilder {
	builder.Component("RPC engine", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		builder.rpcEng = rpc.New(
			node.Logger,
			node.State,
			builder.rpcConf,
			builder.collectionRPC,
			builder.historicalAccessRPCs,
			node.Storage.Blocks,
			node.Storage.Headers,
			node.Storage.Collections,
			node.Storage.Transactions,
			node.Storage.Receipts,
			node.RootChainID,
			builder.transactionMetrics,
			builder.collectionGRPCPort,
			builder.executionGRPCPort,
			builder.retryEnabled,
			builder.rpcMetricsEnabled,
			builder.apiRatelimits,
			builder.apiBurstlimits,
		)
		return builder.rpcEng, nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) ingestionComponent() *FlowAccessNodeBuilder {
	builder.Component("ingestion engine", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		var err error
		builder.requestEng, err = requester.New(
			node.Logger,
			node.Metrics.Engine,
			node.Network,
			node.Me,
			node.State,
			engine.RequestCollections,
			filter.HasRole(flow.RoleCollection),
			func() flow.Entity { return &flow.Collection{} },
		)
		if err != nil {
			return nil, fmt.Errorf("could not create requester engine: %w", err)
		}
		builder.ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Me, builder.requestEng, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, node.Storage.Receipts, builder.transactionMetrics,
			builder.collectionsToMarkFinalized, builder.collectionsToMarkExecuted, builder.blocksToMarkExecuted, builder.rpcEng)
		builder.requestEng.WithHandle(builder.ingestEng.OnCollection)
		return builder.ingestEng, err
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) requesterEngineComponent() *FlowAccessNodeBuilder {
	builder.Component("requester engine", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
		// order for it to properly start and shut down, we should still return it as its own engine here, so it can
		// be handled by the scaffold.
		return builder.requestEng, nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) followerEngineComponent() *FlowAccessNodeBuilder {
	builder.Component("follower engine", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		// initialize cleaner for DB
		cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

		// create a finalizer that will handle updating the protocol
		// state when the follower detects newly finalized blocks
		final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, builder.followerState)

		// initialize the staking & beacon verifiers, signature joiner
		staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
		beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
		merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)

		// initialize consensus committee's membership state
		// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
		// Note: node.Me.NodeID() is not part of the consensus committee
		committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
		if err != nil {
			return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
		}

		// initialize the verifier for the protocol consensus
		verifier := verification.NewCombinedVerifier(committee, staking, beacon, merger)

		finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
		if err != nil {
			return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
		}

		builder.finalizationDistributor = pubsub.NewFinalizationDistributor()
		builder.finalizationDistributor.AddConsumer(builder.ingestEng)

		// creates a consensus follower with ingestEngine as the notifier
		// so that it gets notified upon each new finalized block
		followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier,
			builder.finalizationDistributor, node.RootBlock.Header, node.RootQC, finalized, pending)
		if err != nil {
			return nil, fmt.Errorf("could not initialize follower core: %w", err)
		}

		builder.followerEng, err = followereng.New(
			node.Logger,
			node.Network,
			node.Me,
			node.Metrics.Engine,
			node.Metrics.Mempool,
			cleaner,
			node.Storage.Headers,
			node.Storage.Payloads,
			builder.followerState,
			builder.conCache,
			followerCore,
			builder.syncCore,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create follower engine: %w", err)
		}

		return builder.followerEng, nil
	})
	return builder
}

func (builder *FlowAccessNodeBuilder) syncEngineComponent() *FlowAccessNodeBuilder {
builder.Component("sync engine", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
	sync, err := synceng.New(
		node.Logger,
		node.Metrics.Engine,
		node.Network,
		node.Me,
		node.State,
		node.Storage.Blocks,
		builder.followerEng,
		builder.syncCore,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create synchronization engine: %w", err)
	}

	builder.finalizationDistributor.AddOnBlockFinalizedConsumer(sync.OnFinalizedBlock)

	return sync, nil
})
	return builder
}

func (builder *FlowAccessNodeBuilder) pingEngineComponent() *FlowAccessNodeBuilder {
	builder.Component("ping engine", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		ping, err := pingeng.New(
			node.Logger,
			node.State,
			node.Me,
			builder.pingMetrics,
			builder.pingEnabled,
			node.Middleware,
			builder.nodeInfoFile,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create ping engine: %w", err)
		}
		return ping, nil
	})
	return builder
}
