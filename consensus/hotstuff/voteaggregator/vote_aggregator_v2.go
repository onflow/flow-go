package voteaggregator

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
)

// defaultVoteAggregatorWorkers number of workers to dispatch events for vote aggregators
const defaultVoteAggregatorWorkers = 8

// VoteAggregator stores the votes and aggregates them into a QC when enough votes have been collected
type VoteAggregatorV2 struct {
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

// NewVoteAggregatorV2 creates an instance of vote aggregator
func NewVoteAggregatorV2(notifier hotstuff.Consumer, highestPrunedView uint64, committee hotstuff.Committee, voteValidator hotstuff.Validator, signer hotstuff.SignerVerifier) *VoteAggregatorV2 {
	return &VoteAggregatorV2{
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
func (va *VoteAggregatorV2) Ready() <-chan struct{} {
	// launch as many workers as we need
	for i := 0; i < defaultVoteAggregatorWorkers; i++ {
		va.unit.Launch(va.queuedVotesProcessingLoop)
	}

	return va.unit.Ready()
}

func (va *VoteAggregatorV2) Done() <-chan struct{} {
	return va.unit.Done()
}

func (va *VoteAggregatorV2) queuedVotesProcessingLoop() {
	notifier := va.queuedVotesNotifier.Channel()
	for {
		select {
		case <-va.unit.Quit():
			return
		case <-notifier:
			err := va.processQueuedVoteEvents()
			if err != nil {
				va.log.Fatal().Err(err).Msg("internal error processing block incorporated queued message")
			}
		}
	}
}

func (va *VoteAggregatorV2) processQueuedVoteEvents() error {
	for {
		select {
		case <-va.unit.Quit():
			return nil
		default:
		}

		msg, ok := va.queuedVotes.Pop()
		if ok {
			err := va.processQueuedVote(msg.(*model.Vote))
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

func (va *VoteAggregatorV2) processQueuedVote(vote *model.Vote) error {
	// TODO: log created
	collector, _, err := va.collectors.GetOrCreateCollector(vote.View, vote.BlockID)
	if err != nil {
		return fmt.Errorf("could not lazy init collector for view %d, blockID %v: %w",
			vote.View, vote.BlockID, err)
	}
	err = collector.AddVote(vote)
	if err != nil {
		return fmt.Errorf("could not process vote for view %d, blockID %v: %w",
			vote.View, vote.BlockID, err)
	}

	return nil
}

func (va *VoteAggregatorV2) AddVote(vote *model.Vote) error {
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

func (va *VoteAggregatorV2) AddBlock(block *model.Block) error {
	// check if the block is for a view that has already been pruned (and is thus stale)
	if va.isBlockStale(block) {
		return nil
	}

	err := va.collectors.ProcessBlock(block)
	if err != nil {
		return fmt.Errorf("could not process block %v: %w", block.BlockID, err)
	}

	return nil
}

func (va *VoteAggregatorV2) InvalidBlock(block *model.Block) {
	panic("implement me")
}

// PruneUpToView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregatorV2) PruneUpToView(view uint64) {
	// if someone else has updated view in parallel don't bother doing extra work for cleaning, whoever
	// is able to advance counter will perform the cleanup
	if va.highestPrunedView.Set(view) {
		err := va.collectors.PruneUpToView(view)
		if err != nil {
			va.log.Fatal().Err(err).Msgf("fatal error when pruning vote collectors by view %d", view)
		}
	}
}

func (va *VoteAggregatorV2) isVoteStale(vote *model.Vote) bool {
	return vote.View <= va.highestPrunedView.Value()
}

func (va *VoteAggregatorV2) isBlockStale(block *model.Block) bool {
	return block.View <= va.highestPrunedView.Value()
}
