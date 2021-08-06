package consensus_follower

import (
	"context"

	"github.com/reactivex/rxgo/v2"

	"github.com/onflow/flow-go/cmd"
	access "github.com/onflow/flow-go/cmd/access"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
)

func finalizedBlockProducer(anb *access.UnstakedAccessNodeBuilder) rxgo.Producer {
	return func(ctx context.Context, next chan<- rxgo.Item) {
		anb.FinalizationDistributor.AddOnBlockFinalizedConsumer(onBlockFinalizedConsumer(next))

		select {
		case <-anb.Ready():
			// log: component startup complete
		case <-ctx.Done():
			// log: component startup aborted
			return
		}

		<-ctx.Done()

		// log: shutting down

		<-anb.Done()
	}
}

func onBlockFinalizedConsumer(next chan<- rxgo.Item) pubsub.OnBlockFinalizedConsumer {
	return func(finalizedBlockID flow.Identifier) {
		next <- rxgo.Of(finalizedBlockID)
	}
}

func buildAccessNode(accessNodeOptions []access.Option) *access.UnstakedAccessNodeBuilder {
	anb := access.FlowAccessNode(accessNodeOptions...)
	nodeBuilder := access.NewUnstakedAccessNodeBuilder(anb)

	nodeBuilder.Initialize()
	nodeBuilder.BuildConsensusFollower()

	return nodeBuilder
}

type Option func(c *Config)

func getAccessNodeOptions(config *Config) []access.Option {
	return []access.Option{
		access.WithUpstreamAccessNodeID(config.upstreamAccessNodeID),
		access.WithUnstakedNetworkBindAddr(config.bindAddr),
		access.WithBaseOptions(getBaseOptions(config)),
	}
}

func getBaseOptions(config *Config) []cmd.Option {
	options := []cmd.Option{cmd.WithNodeID(config.nodeID)}
	if config.bootstrapDir != "" {
		options = append(options, cmd.WithBootstrapDir(config.bootstrapDir))
	}
	if config.dataDir != "" {
		options = append(options, cmd.WithDataDir(config.dataDir))
	}

	return options
}

func GetFinalizedBlockProducer(
	nodeID flow.Identifier,
	upstreamAccessNodeID flow.Identifier,
	bindAddr string,
	opts ...Option,
) rxgo.Producer {
	config := &Config{
		nodeID:               nodeID,
		upstreamAccessNodeID: upstreamAccessNodeID,
		bindAddr:             bindAddr,
	}

	for _, opt := range opts {
		opt(config)
	}

	accessNodeOptions := getAccessNodeOptions(config)
	anb := buildAccessNode(accessNodeOptions)

	return finalizedBlockProducer(anb)
}

func CreateFinalizedBlockObservable(
	nodeID flow.Identifier,
	upstreamAccessNodeID flow.Identifier,
	bindAddr string,
	opts ...Option,
) rxgo.Observable {
	return rxgo.Create([]rxgo.Producer{GetFinalizedBlockProducer(nodeID, upstreamAccessNodeID, bindAddr, opts...)})
}

type Config struct {
	nodeID               flow.Identifier // the node ID of this node
	upstreamAccessNodeID flow.Identifier // the node ID of the upstream access node
	bindAddr             string          // address to bind on
	dataDir              string          // directory to store the protocol state
	bootstrapDir         string          // path to the bootstrap directory
}
