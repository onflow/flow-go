package voteaggregator

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
)

// defaultVoteAggregatorWorkers number of workers to dispatch events for vote aggregators
const defaultVoteAggregatorWorkers = 8

// defaultVoteQueueCapacity maximum capacity of buffering unprocessed votes
const defaultVoteQueueCapacity = 1000

// VoteAggregator stores the votes and aggregates them into a QC when enough votes have been collected
// VoteAggregator is designed in a way that it can aggregate votes for collection & consensus clusters
// that is why implementation relies on dependency injection.
type VoteAggregator struct {
	*component.ComponentManager
	log                 zerolog.Logger
	notifier            hotstuff.Consumer
	highestPrunedView   counters.StrictMonotonousCounter
	collectors          hotstuff.VoteCollectors
	queuedVotesNotifier engine.Notifier
	queuedVotes         *fifoqueue.FifoQueue
}

var _ hotstuff.VoteAggregatorV2 = (*VoteAggregator)(nil)
var _ module.ReadyDoneAware = (*VoteAggregator)(nil)
var _ module.Startable = (*VoteAggregator)(nil)

// NewVoteAggregator creates an instance of vote aggregator
// Note: verifyingProcessorFactory is injected. Thereby, the code is agnostic to the
// different voting formats of main Consensus vs Collector consensus.
func NewVoteAggregator(
	log zerolog.Logger,
	notifier hotstuff.Consumer,
	highestPrunedView uint64,
	collectors hotstuff.VoteCollectors,
) (*VoteAggregator, error) {

	queuedVotes, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultVoteQueueCapacity))
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}

	aggregator := &VoteAggregator{
		log:                 log,
		notifier:            notifier,
		highestPrunedView:   counters.NewMonotonousCounter(highestPrunedView),
		collectors:          collectors,
		queuedVotes:         queuedVotes,
		queuedVotesNotifier: engine.NewNotifier(),
	}

	componentBuilder := component.NewComponentManagerBuilder()

	// launch as many workers as we need
	for i := 0; i < defaultVoteAggregatorWorkers; i++ {
		componentBuilder.AddWorker(aggregator.queuedVotesProcessingLoop)
	}

	aggregator.ComponentManager = componentBuilder.Build()

	return aggregator, nil
}

func (va *VoteAggregator) queuedVotesProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	notifier := va.queuedVotesNotifier.Channel()
	ready() // signal that this worker is ready
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := va.processQueuedVoteEvents(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued vote events: %w", err))
				return
			}
		}
	}
}

func (va *VoteAggregator) processQueuedVoteEvents(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
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

// processQueuedVote performs actual processing of queued votes, this method is called from multiple
// concurrent goroutines.
func (va *VoteAggregator) processQueuedVote(vote *model.Vote) error {
	// TODO: log created
	collector, _, err := va.collectors.GetOrCreateCollector(vote.View)
	if err != nil {
		// ignore if our routine is outdated and some other one has pruned collectors
		if mempool.IsDecreasingPruningHeightError(err) {
			return nil
		}
		return fmt.Errorf("could not get collector for view %d: %w",
			vote.View, err)
	}
	err = collector.AddVote(vote)
	if err != nil {
		if model.IsDoubleVoteError(err) {
			doubleVoteErr := err.(model.DoubleVoteError)
			va.notifier.OnDoubleVotingDetected(doubleVoteErr.FirstVote, doubleVoteErr.ConflictingVote)
			return nil
		}

		return fmt.Errorf("could not process vote for view %d, blockID %v: %w",
			vote.View, vote.BlockID, err)
	}

	return nil
}

// AddVote checks if vote is stale and appends vote into processing queue
// actual vote processing will be called in other dispatching goroutine.
func (va *VoteAggregator) AddVote(vote *model.Vote) {
	// drop stale votes
	if vote.View <= va.highestPrunedView.Value() {
		return
	}

	// It's ok to silently drop votes in case our processing pipeline is full.
	// It means that we are probably catching up.
	if ok := va.queuedVotes.Push(vote); ok {
		va.queuedVotesNotifier.Notify()
	}
}

// AddBlock notifies the VoteAggregator about a known block so that it can start processing
// pending votes whose block was unknown.
// It also verifies the proposer vote of a block, and return whether the proposer signature is valid.
// Expected error returns during normal operations:
// * model.InvalidBlockError if the proposer's vote for its own block is invalid
// * mempool.DecreasingPruningHeightError if the block's view has already been pruned
func (va *VoteAggregator) AddBlock(block *model.Proposal) error {
	// check if the block is for a view that has already been pruned (and is thus stale)
	if block.Block.View <= va.highestPrunedView.Value() {
		return mempool.NewDecreasingPruningHeightErrorf("block proposal for view %d is stale, highestPrunedView is %d", block.Block.View, va.highestPrunedView.Value())
	}

	collector, _, err := va.collectors.GetOrCreateCollector(block.Block.View)
	if err != nil {
		return fmt.Errorf("could not get or create collector for block %v: %w", block.Block.BlockID, err)
	}

	err = collector.ProcessBlock(block)
	if err != nil {
		return fmt.Errorf("could not process block: %v, %w", block.Block.BlockID, err)
	}

	return nil
}

// InvalidBlock notifies the VoteAggregator about an invalid proposal, so that it
// can process votes for the invalid block and slash the voters. Expected error
// returns during normal operations:
// * mempool.DecreasingPruningHeightError if proposal's view has already been pruned
func (va *VoteAggregator) InvalidBlock(proposal *model.Proposal) error {
	slashingVoteConsumer := func(vote *model.Vote) {
		if proposal.Block.BlockID == vote.BlockID {
			va.notifier.OnVoteForInvalidBlockDetected(vote, proposal)
		}
	}

	block := proposal.Block
	collector, _, err := va.collectors.GetOrCreateCollector(block.View)
	if err != nil {
		// ignore if our routine is outdated and some other one has pruned collectors
		if mempool.IsDecreasingPruningHeightError(err) {
			return nil
		}
		return fmt.Errorf("could not retrieve vote collector for view %d: %w", block.View, err)
	}
	// registering vote consumer will deliver all previously cached votes in strict order
	// and will keep delivering votes if more are collected
	collector.RegisterVoteConsumer(slashingVoteConsumer)
	return nil
}

// PruneUpToView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneUpToView(view uint64) {
	// if someone else has updated view in parallel don't bother doing extra work for cleaning, whoever
	// is able to advance counter will perform the cleanup
	if va.highestPrunedView.Set(view) {
		va.collectors.PruneUpToView(view)
	}
}
