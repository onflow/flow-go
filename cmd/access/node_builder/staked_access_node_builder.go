package node_builder

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/cmd"
	pingeng "github.com/onflow/flow-go/engine/access/ping"
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
		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, fnb.ProtocolEvents)
		if err != nil {
			return err
		}

		fnb.IdentityProvider = idCache
		// translator
		fnb.SyncEngineParticipantsProvider = id.NewFilteredIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(node.Me.NodeID())),
				p2p.NotEjectedFilter,
			),
			idCache,
		)
		fnb.IDTranslator = p2p.NewHierarchicalIDTranslator(idCache, p2p.NewUnstakedNetworkIDTranslator())

		// TODO: NetworkingIdentifierProvider should be the same as the one used in scaffold.go if this AN
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
	// TODO: we should remove this call since we are no longer instantiating two networks
	// builder.EnqueueNetworkInit(ctx)

	// if this is upstream staked AN for unstaked ANs, initialize the network to communicate on the unstaked network
	if builder.ParticipatesInUnstakedNetwork() {
		builder.enqueueUnstakedNetworkInit(ctx)
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

		// NodeID for the staked node on the unstaked network
		// TODO: set a different node ID of the staked access node on the unstaked network
		unstakedNodeID := builder.NodeID // currently set the same as the staked NodeID

		// Networking key
		// TODO: set a different networking key of the staked access node on the unstaked network
		unstakedNetworkKey := builder.NetworkKey

		libP2PFactory, err := builder.initLibP2PFactory(ctx, unstakedNodeID, unstakedNetworkKey)
		builder.MustNot(err)

		msgValidators := unstakedNetworkMsgValidators(unstakedNodeID)

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		// TODO: define new network metrics for the unstaked network
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		middleware := builder.initMiddleware(unstakedNodeID, unstakedNetworkMetrics, libP2PFactory, msgValidators...)

		// topology returns empty list since peers are not known upfront
		top := topology.EmptyListTopology{}

		network, err := builder.initNetwork(builder.Me, unstakedNetworkMetrics, middleware, top)
		builder.MustNot(err)

		builder.UnstakedNetwork = network
		builder.unstakedMiddleware = middleware

		node.Logger.Info().Msgf("unstaked network will run on address: %s", builder.unstakedNetworkBindAddr)
		return builder.UnstakedNetwork, err
	})
}
