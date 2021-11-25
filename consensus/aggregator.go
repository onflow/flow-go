package consensus

import (
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
)

type HotstuffModules struct {
	Forks                hotstuff.Forks
	Validator            hotstuff.Validator
	Notifier             hotstuff.Consumer
	Committee            hotstuff.Committee
	Signer               hotstuff.SignerVerifier
	Persist              hotstuff.Persister
	Aggregator           hotstuff.VoteAggregator
	QCCreatedDistributor *pubsub.QCCreatedDistributor
}

func NewVoteAggregator(
	log zerolog.Logger,
	finalized *flow.Header,
	pending []*flow.Header,
	modules *HotstuffModules,
	workerPool *workerpool.WorkerPool,
	voteProcessorFactory hotstuff.VoteProcessorFactory,
) (hotstuff.VoteAggregator, error) {

	createCollectorFactoryMethod := func(view uint64) (hotstuff.VoteCollector, error) {
		return votecollector.NewStateMachine(view, log, workerPool, modules.Notifier, voteProcessorFactory.Create), nil
	}

	// get the last view we started
	started, err := modules.Persist.GetStarted()
	if err != nil {
		return nil, fmt.Errorf("could not recover last started: %w", err)
	}

	voteCollectors := voteaggregator.NewVoteCollectors(started, createCollectorFactoryMethod)

	// initialize the vote aggregator
	aggregator, err := voteaggregator.NewVoteAggregator(log, modules.Notifier, started, voteCollectors)
	if err != nil {
		return nil, fmt.Errorf("could not create vote aggregator: %w", err)
	}

	// recover the hotstuff state, mainly to recover all pending blocks
	// in Forks
	err = recovery.Participant(log, modules.Forks, aggregator, modules.Validator, finalized, pending)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff state: %w", err)
	}

	return aggregator, nil
}
