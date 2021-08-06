package consensus_follower

import (
	"context"

	"github.com/reactivex/rxgo/v2"

	access "github.com/onflow/flow-go/cmd/access"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
)

func finalizedBlockProducer(anb *access.UnstakedAccessNodeBuilder) rxgo.Producer {
	return func(ctx context.Context, next chan<- rxgo.Item) {
		anb.FinalizationDistributor.AddOnBlockFinalizedConsumer(onBlockFinalizedConsumer(next))
		runAccessNode(ctx, anb)
	}
}

func onBlockFinalizedConsumer(next chan<- rxgo.Item) pubsub.OnBlockFinalizedConsumer {
	return func(finalizedBlockID flow.Identifier) {
		next <- rxgo.Of(finalizedBlockID)
	}
}

// GetFinalizedBlockProducer returns a `rxgo.Producer` for block finalization events.
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

// CreateFinalizedBlockObservable returns a `rxgo.Observable` representing a stream of block
// finalization events.
func CreateFinalizedBlockObservable(
	nodeID flow.Identifier,
	upstreamAccessNodeID flow.Identifier,
	bindAddr string,
	opts ...Option,
) rxgo.Observable {
	return rxgo.Create([]rxgo.Producer{GetFinalizedBlockProducer(nodeID, upstreamAccessNodeID, bindAddr, opts...)})
}
