package consensus

import (
	"fmt"
	"github.com/onflow/flow-go/module"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
)

// NewVoteAggregator creates new VoteAggregator and subscribes for finalization events.
// No error returns are expected during normal operations.
func NewVoteAggregator(
	log zerolog.Logger,
	lowestRetainedView uint64,
	notifier hotstuff.Consumer,
	mempoolMetrics module.MempoolMetrics,
	voteProcessorFactory hotstuff.VoteProcessorFactory,
	distributor *pubsub.FinalizationDistributor,
) (hotstuff.VoteAggregator, error) {

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, notifier, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(log, lowestRetainedView, workerpool.New(4), createCollectorFactoryMethod)

	// initialize the vote aggregator
	aggregator, err := voteaggregator.NewVoteAggregator(log, notifier, mempoolMetrics, lowestRetainedView, voteCollectors)
	if err != nil {
		return nil, fmt.Errorf("could not create vote aggregator: %w", err)
	}
	distributor.AddOnBlockFinalizedConsumer(aggregator.OnFinalizedBlock)

	return aggregator, nil
}

// NewTimeoutAggregator creates new TimeoutAggregator and connects Hotstuff event source with event handler.
// No error returns are expected during normal operations.
func NewTimeoutAggregator(log zerolog.Logger,
	lowestRetainedView uint64,
	notifier *pubsub.Distributor,
	mempoolMetrics module.MempoolMetrics,
	timeoutProcessorFactory hotstuff.TimeoutProcessorFactory,
	distributor *pubsub.TimeoutCollectorDistributor,
) (hotstuff.TimeoutAggregator, error) {

	timeoutCollectorFactory := timeoutcollector.NewTimeoutCollectorFactory(notifier, distributor, timeoutProcessorFactory)
	collectors := timeoutaggregator.NewTimeoutCollectors(log, lowestRetainedView, timeoutCollectorFactory)

	// initialize the timeout aggregator
	aggregator, err := timeoutaggregator.NewTimeoutAggregator(log, notifier, mempoolMetrics, lowestRetainedView, collectors)
	if err != nil {
		return nil, fmt.Errorf("could not create timeout aggregator: %w", err)
	}
	notifier.AddConsumer(aggregator)

	return aggregator, nil
}
