package consensus

import (
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/module"
)

// NewVoteAggregator creates new VoteAggregator and subscribes for finalization events.
// No error returns are expected during normal operations.
func NewVoteAggregator(
	log zerolog.Logger,
	hotstuffMetrics module.HotstuffMetrics,
	engineMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	lowestRetainedView uint64,
	notifier hotstuff.VoteAggregationConsumer,
	voteProcessorFactory hotstuff.VoteProcessorFactory,
	distributor *pubsub.FollowerDistributor,
) (hotstuff.VoteAggregator, error) {

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, notifier, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(log, lowestRetainedView, workerpool.New(4), createCollectorFactoryMethod)

	// initialize the vote aggregator
	aggregator, err := voteaggregator.NewVoteAggregator(
		log,
		hotstuffMetrics,
		engineMetrics,
		mempoolMetrics,
		notifier,
		lowestRetainedView,
		voteCollectors,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create vote aggregator: %w", err)
	}
	distributor.AddOnBlockFinalizedConsumer(aggregator.OnFinalizedBlock)

	return aggregator, nil
}

// NewTimeoutAggregator creates new TimeoutAggregator and connects Hotstuff event source with event handler.
// No error returns are expected during normal operations.
func NewTimeoutAggregator(log zerolog.Logger,
	hotstuffMetrics module.HotstuffMetrics,
	engineMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	notifier *pubsub.Distributor,
	timeoutProcessorFactory hotstuff.TimeoutProcessorFactory,
	distributor *pubsub.TimeoutAggregationDistributor,
	lowestRetainedView uint64,
) (hotstuff.TimeoutAggregator, error) {

	timeoutCollectorFactory := timeoutcollector.NewTimeoutCollectorFactory(log, distributor, timeoutProcessorFactory)
	collectors := timeoutaggregator.NewTimeoutCollectors(log, hotstuffMetrics, lowestRetainedView, timeoutCollectorFactory)

	// initialize the timeout aggregator
	aggregator, err := timeoutaggregator.NewTimeoutAggregator(
		log,
		hotstuffMetrics,
		engineMetrics,
		mempoolMetrics,
		lowestRetainedView,
		collectors,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create timeout aggregator: %w", err)
	}
	notifier.AddConsumer(aggregator)

	return aggregator, nil
}
