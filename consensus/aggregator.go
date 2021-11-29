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

// HotstuffModules is a helper structure which helps to initialize hotstuff modules
// It contains main dependencies that are needed to inittialize hotstuff hotstuff.VoteAggregator and hotstuff.EventLoop
type HotstuffModules struct {
	Forks                hotstuff.Forks               // information about multiple forks
	Validator            hotstuff.Validator           // validator of proposals & votes
	Notifier             hotstuff.Consumer            // observer for hotstuff events
	Committee            hotstuff.Committee           // consensus committee
	Signer               hotstuff.SignerVerifier      // signer of proposal & votes
	Persist              hotstuff.Persister           // last state of consensus participant
	Aggregator           hotstuff.VoteAggregator      // aggregator of votes, used by leader
	QCCreatedDistributor *pubsub.QCCreatedDistributor // observer for qc created event, used by leader
}

// NewVoteAggregator creates new VoteAggregator and recover the Forks' state with all pending block
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
