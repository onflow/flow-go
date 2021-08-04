package main

import (
	"fmt"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/access/ingestion"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/state/protocol"
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

	// IsStaked returns True is this is a staked Access Node, False otherwise
	IsStaked() bool

	// ParticipatesInUnstakedNetwork returns True if this is a staked Access node which also participates
	// in the unstaked network acting as an upstream for other unstaked access nodes, False otherwise.
	ParticipatesInUnstakedNetwork() bool
}

// AccessNodeConfig defines all the user defined parameters required to bootstrap an access node
// For a node running as a standalone process, the config fields will be populated from the command line params,
// while for a node running as a library, the config fields are expected to be initialized by the caller.
type AccessNodeConfig struct {
	staked                       bool
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
	rpcConf                      rpc.Config
	ExecutionNodeAddress         string // deprecated
	HistoricalAccessRPCs         []access.AccessAPIClient
	logTxTimeToFinalized         bool
	logTxTimeToExecuted          bool
	logTxTimeToFinalizedExecuted bool
	retryEnabled                 bool
	rpcMetricsEnabled            bool
	baseOptions                  []cmd.Option
}

// DefaultAccessNodeConfig defines all the default values for the AccessNodeConfig
func DefaultAccessNodeConfig() *AccessNodeConfig {
	return &AccessNodeConfig{
		receiptLimit:       1000,
		collectionLimit:    1000,
		blockLimit:         1000,
		collectionGRPCPort: 9000,
		executionGRPCPort:  9000,
		rpcConf: rpc.Config{
			UnsecureGRPCListenAddr:    "localhost:9000",
			SecureGRPCListenAddr:      "localhost:9001",
			HTTPListenAddr:            "localhost:8000",
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
		stakedAccessNodeIDHex:        "",
		unstakedNetworkBindAddr:      cmd.NotSet,
	}
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow staked and unstaked access node
// It is composed of the FlowNodeBuilder, the AccessNodeConfig and contains all the components and modules needed for the
// staked and unstaked access nodes
type FlowAccessNodeBuilder struct {
	*cmd.FlowNodeBuilder
	*AccessNodeConfig

	// components
	UnstakedNetwork            *p2p.Network
	unstakedMiddleware         *p2p.Middleware
	FollowerState              protocol.MutableState
	SyncCore                   *synchronization.Core
	RpcEng                     *rpc.Engine
	FinalizationDistributor    *pubsub.FinalizationDistributor
	FinalizedHeader            *synceng.FinalizedHeaderCache
	CollectionRPC              access.AccessAPIClient
	ConCache                   *buffer.PendingBlocks // pending block cache for follower
	TransactionTimings         *stdmap.TransactionTimings
	CollectionsToMarkFinalized *stdmap.Times
	CollectionsToMarkExecuted  *stdmap.Times
	BlocksToMarkExecuted       *stdmap.Times
	TransactionMetrics         module.TransactionMetrics
	PingMetrics                module.PingMetrics

	// engines
	IngestEng   *ingestion.Engine
	RequestEng  *requester.Engine
	FollowerEng *followereng.Engine
}

type Option func(*AccessNodeConfig)

func WithUpstreamAccessNodeID(upstreamAccessNodeID flow.Identifier) Option {
	return func(config *AccessNodeConfig) {
		config.stakedAccessNodeIDHex = upstreamAccessNodeID.String()
	}
}

func WithUnstakedNetworkBindAddr(bindAddr string) Option {
	return func(config *AccessNodeConfig) {
		config.unstakedNetworkBindAddr = bindAddr
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
		AccessNodeConfig: config,
		FlowNodeBuilder:  cmd.FlowNode(flow.RoleAccess.String(), config.baseOptions...),
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
