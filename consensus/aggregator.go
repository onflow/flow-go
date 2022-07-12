package consensus

import (
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/model/flow"
)

// NewVoteAggregator creates new VoteAggregator and recover the Forks' state with all pending block
func NewVoteAggregator(
	log zerolog.Logger,
	lowestRetainedView uint64,
	notifier hotstuff.Consumer,
	voteProcessorFactory hotstuff.VoteProcessorFactory,
	distributor *pubsub.FinalizationDistributor,
) (hotstuff.VoteAggregator, error) {

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, notifier, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(log, lowestRetainedView, workerpool.New(4), createCollectorFactoryMethod)

	// initialize the vote aggregator
	aggregator, err := voteaggregator.NewVoteAggregator(log, notifier, lowestRetainedView, voteCollectors)
	if err != nil {
		return nil, fmt.Errorf("could not create vote aggregator: %w", err)
	}
	distributor.AddOnBlockFinalizedConsumer(aggregator.OnFinalizedBlock)

	return aggregator, nil
}

// timeoutAggregatorConsumerAdapter is an utilitity function that serves as adapter for hotstuff.Consumer which overrides
// events and forwards them to `TimeoutAggregator`.
type timeoutAggregatorConsumerAdapter struct {
	notifications.NoopConsumer
	aggregator *timeoutaggregator.TimeoutAggregator
}

func (t *timeoutAggregatorConsumerAdapter) OnEnteringView(viewNumber uint64, leader flow.Identifier) {
	t.aggregator.OnEnteringView(viewNumber, leader)
}

var _ hotstuff.Consumer = (*timeoutAggregatorConsumerAdapter)(nil)

// NewTimeoutAggregator creates new TimeoutAggregator and connects Hotstuff event source with event handler
func NewTimeoutAggregator(log zerolog.Logger,
	lowestRetainedView uint64,
	notifier *pubsub.Distributor,
	timeoutProcessorFactory hotstuff.TimeoutProcessorFactory,
	distributor *pubsub.TimeoutCollectorDistributor,
) (hotstuff.TimeoutAggregator, error) {

	timeoutCollectorFactory := timeoutcollector.NewTimeoutCollectorFactory(notifier, distributor, timeoutProcessorFactory)
	collectors := timeoutaggregator.NewTimeoutCollectors(log, lowestRetainedView, timeoutCollectorFactory)

	// initialize the timeout aggregator
	aggregator, err := timeoutaggregator.NewTimeoutAggregator(log, notifier, lowestRetainedView, collectors)
	if err != nil {
		return nil, fmt.Errorf("could not create timeout aggregator: %w", err)
	}
	adapter := &timeoutAggregatorConsumerAdapter{
		aggregator: aggregator,
	}
	notifier.AddConsumer(adapter)

	return aggregator, nil
}
