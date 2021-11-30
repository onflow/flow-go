package consensus

import (
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
)

// NewVoteAggregator creates new VoteAggregator and recover the Forks' state with all pending block
func NewVoteAggregator(
	log zerolog.Logger,
	finalized *flow.Header,
	pending []*flow.Header,
	notifier hotstuff.Consumer,
	forks hotstuff.Forks,
	validator hotstuff.Validator,
	voteProcessorFactory hotstuff.VoteProcessorFactory,
) (hotstuff.VoteAggregator, error) {

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, notifier, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(finalized.View, workerpool.New(4), createCollectorFactoryMethod)

	// initialize the vote aggregator
	aggregator, err := voteaggregator.NewVoteAggregator(log, notifier, finalized.View, voteCollectors)
	if err != nil {
		return nil, fmt.Errorf("could not create vote aggregator: %w", err)
	}

	// recover the hotstuff state, mainly to recover all pending blocks in Forks
	err = recovery.Participant(log, forks, aggregator, validator, finalized, pending)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff state: %w", err)
	}

	return aggregator, nil
}
