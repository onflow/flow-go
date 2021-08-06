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

		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-anb.Ready():
			// log: startup complete
		case <-ctx.Done():
			// log: startup aborted
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
