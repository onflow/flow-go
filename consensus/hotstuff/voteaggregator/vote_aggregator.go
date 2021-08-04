package voteaggregator

import (
	"fmt"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// defaultVoteAggregatorWorkers number of workers to dispatch events for vote aggregators
const defaultVoteAggregatorWorkers = 8

// VoteAggregator stores the votes and aggregates them into a QC when enough votes have been collected
type VoteAggregator struct {
	unit                *engine.Unit
	log                 zerolog.Logger
	notifier            hotstuff.Consumer
	committee           hotstuff.Committee
	voteValidator       hotstuff.Validator
	signer              hotstuff.SignerVerifier
	highestPrunedView   counters.StrictMonotonousCounter
	collectors          VoteCollectors
	queuedVotesNotifier engine.Notifier
	queuedVotes         *fifoqueue.FifoQueue // keeps track of votes whose blocks can not be found
}

// New creates an instance of vote aggregator
func New(notifier hotstuff.Consumer, highestPrunedView uint64, committee hotstuff.Committee, voteValidator hotstuff.Validator, signer hotstuff.SignerVerifier) *VoteAggregator {
	return &VoteAggregator{
		notifier:          notifier,
		highestPrunedView: counters.NewMonotonousCounter(highestPrunedView),
		committee:         committee,
		voteValidator:     voteValidator,
		signer:            signer,
		unit:              engine.NewUnit(),
	}
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (va *VoteAggregator) Ready() <-chan struct{} {
	// launch as many workers as we need
	for i := 0; i < defaultVoteAggregatorWorkers; i++ {
		va.unit.Launch(va.pendingVotesProcessingLoop)
	}

	return va.unit.Ready()
}

func (va *VoteAggregator) Done() <-chan struct{} {
	return va.unit.Done()
}

func (va *VoteAggregator) pendingVotesProcessingLoop() {
	notifier := va.queuedVotesNotifier.Channel()
	for {
		select {
		case <-va.unit.Quit():
			return
		case <-notifier:
			err := va.processPendingVoteEvents()
			if err != nil {
				va.log.Fatal().Err(err).Msg("internal error processing block incorporated queued message")
			}
		}
	}
}

func (va *VoteAggregator) processPendingVoteEvents() error {
	for {
		select {
		case <-va.unit.Quit():
			return nil
		default:
		}

		msg, ok := va.queuedVotes.Pop()
		if ok {
			err := va.processPendingVote(msg.(*model.Vote))
			if err != nil {
				return fmt.Errorf("could not process pending vote: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

func (va *VoteAggregator) processPendingVote(vote *model.Vote) error {
	lazyInitCollector, err := va.collectors.GetOrCreateCollector(vote.View, vote.BlockID)
	if err != nil {
		return fmt.Errorf("could not lazy init collector for view %d, blockID %v: %w",
			vote.View, vote.BlockID, err)
	}
	err = lazyInitCollector.AddVote(vote)
	if err != nil {
		return fmt.Errorf("could not process vote for view %d, blockID %v: %w",
			vote.View, vote.BlockID, err)
	}

	return nil
}

func (va *VoteAggregator) AddVote(vote *model.Vote) error {
	// drop stale votes
	if va.isVoteStale(vote) {
		return nil
	}

	// It's ok to silently drop votes in case our processing pipeline is full.
	// It means that we are probably catching up.
	if ok := va.queuedVotes.Push(vote); ok {
		va.queuedVotesNotifier.Notify()
	}

	return nil
}

func (va *VoteAggregator) AddBlock(block *model.Block) error {
	// check if the block is for a view that has already been pruned (and is thus stale)
	if va.isBlockStale(block) {
		return nil
	}

	// TODO: check block signature

	err := va.collectors.ProcessBlock(block)
	if err != nil {
		return fmt.Errorf("could not process block %v: %w", block.BlockID, err)
	}

	// after this call, collector might change state

	return nil
}

func (va *VoteAggregator) GetVoteCreator(block *model.Block) (hotstuff.CreateVote, error) {
	panic("implement me")
}

func (va *VoteAggregator) InvalidBlock(block *model.Block) {
	panic("implement me")
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	// if someone else has updated view in parallel don't bother doing extra work for cleaning, whoever
	// is able to advance counter will perform the cleanup
	if va.highestPrunedView.Set(view) {
		err := va.collectors.PruneByView(view)
		if err != nil {
			va.log.Fatal().Err(err).Msgf("fatal error when pruning vote collectors by view %d", view)
		}
	}
}

func (va *VoteAggregator) isVoteStale(vote *model.Vote) bool {
	return vote.View <= va.highestPrunedView.Value()
}

func (va *VoteAggregator) isBlockStale(block *model.Block) bool {
	return block.View <= va.highestPrunedView.Value()
}
