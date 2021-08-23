package node_builder

import (
	"context"
	"fmt"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/crypto"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
	"github.com/onflow/flow-go/engine/access/relay"
	splitternetwork "github.com/onflow/flow-go/engine/common/splitter/network"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
)

// StakedAccessNodeBuilder builds a staked access node. The staked access node can optionally participate in the
// unstaked network publishing data for the unstaked access node downstream.
type StakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func NewStakedAccessNodeBuilder(anb *FlowAccessNodeBuilder) *StakedAccessNodeBuilder {
	return &StakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

func (fnb *StakedAccessNodeBuilder) InitIDProviders() {
	fnb.Module("id providers", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {

		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, node.ProtocolEvents)
		if err != nil {
			return err
		}

		fnb.IdentityProvider = idCache
		fnb.SyncEngineIdentifierProvider = id.NewFilteredIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(node.Me.NodeID())),
				p2p.NotEjectedFilter,
			),
			idCache,
		)

		fnb.IDTranslator = p2p.NewHierarchicalIDTranslator(idCache, p2p.NewUnstakedNetworkIDTranslator())

		// TODO: NetworkingIdentifierProvidzer should be the same as the one used in scaffold.go if this AN
		// doesn't participate in unstaked network.
		// If it does, then we can just use the default one (peerstoreProvider)

		return nil
	})
}

func (builder *StakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	ctx, cancel := context.WithCancel(context.Background())
	builder.Cancel = cancel

	builder.InitIDProviders()

	// for the staked access node, initialize the network used to communicate with the other staked flow nodes
	// by calling the EnqueueNetworkInit on the base FlowBuilder like any other staked node
	builder.EnqueueNetworkInit(ctx)

	// if this is upstream staked AN for unstaked ANs, initialize the network to communicate on the unstaked network
	if builder.SupportsUnstakedNetwork() {
		builder.enqueueUnstakedNetworkInit(ctx)
	} else {
		builder.EnqueueNetworkInit(ctx)
	}

	builder.EnqueueMetricsServerInit()

	builder.RegisterBadgerMetrics()

	builder.EnqueueTracer()

	return builder
}

func (anb *StakedAccessNodeBuilder) Build() AccessNodeBuilder {
	anb.FlowAccessNodeBuilder.
		Build().
		Component("ping engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			ping, err := pingeng.New(
				node.Logger,
				node.State,
				node.Me,
				anb.PingMetrics,
				anb.pingEnabled,
				node.Middleware,
				anb.nodeInfoFile,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create ping engine: %w", err)
			}
			return ping, nil
		})

	return anb
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the staked node
func (builder *StakedAccessNodeBuilder) enqueueUnstakedNetworkInit(ctx context.Context) {

	builder.Component("unstaked network", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		libP2PFactory, err := builder.initLibP2PFactory(ctx, builder.NodeID, builder.NetworkKey)
		builder.MustNot(err)

		msgValidators := unstakedNetworkMsgValidators(builder.NodeID)

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		// TODO: define new network metrics for the unstaked network
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		middleware := builder.initMiddleware(builder.NodeID, unstakedNetworkMetrics, libP2PFactory, msgValidators...)

		// topology returns empty list since peers are not known upfront
		top, err := topology.NewTopicBasedTopology(
			builder.NodeID,
			builder.Logger,
			builder.State,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create topology: %w", err)
		}
		topologyCache := topology.NewCache(builder.Logger, top)

		network, err := builder.initNetwork(builder.Me, unstakedNetworkMetrics, middleware, topologyCache)
		builder.MustNot(err)

		builder.Network = network
		builder.Middleware = middleware

		node.Logger.Info().Msgf("network will run on address: %s", builder.BindAddr)
		return builder.Network, err
	})
}

// initLibP2PFactory creates the LibP2P factory function for the given node ID and network key.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
func (builder *StakedAccessNodeBuilder) initLibP2PFactory(ctx context.Context,
	nodeID flow.Identifier,
	networkKey crypto.PrivateKey) (p2p.LibP2PFactoryFunc, error) {

	// The staked nodes act as the DHT servers
	dhtOptions := []dht.Option{p2p.AsServer(builder.IsStaked())}

	myAddr := builder.NodeConfig.Me.Address()
	if builder.BaseConfig.BindAddr != cmd.NotSet {
		myAddr = builder.BaseConfig.BindAddr
	}

	connManager := p2p.NewConnManager(builder.Logger, builder.Metrics.Network)

	return func() (*p2p.Node, error) {
		libp2pNode, err := p2p.NewDefaultLibP2PNodeBuilder(nodeID, myAddr, networkKey).
			SetRootBlockID(builder.RootBlock.ID().String()).
			// no connection gater
			SetConnectionManager(connManager).
			// act as a DHT server
			SetDHTOptions(dhtOptions...).
			SetPubsubOptions(p2p.DefaultPubsubOptions(p2p.DefaultMaxPubSubMsgSize)...).
			SetLogger(builder.Logger).
			Build(ctx)
		if err != nil {
			return nil, err
		}
		builder.LibP2PNode = libp2pNode
		return builder.LibP2PNode, nil
	}, nil
}
