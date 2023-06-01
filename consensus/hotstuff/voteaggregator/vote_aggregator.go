package voteaggregator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
)

// defaultVoteAggregatorWorkers number of workers to dispatch events for vote aggregators
const defaultVoteAggregatorWorkers = 8

// defaultVoteQueueCapacity maximum capacity of buffering unprocessed votes
const defaultVoteQueueCapacity = 1000

// defaultBlockQueueCapacity maximum capacity of buffering unprocessed blocks
const defaultBlockQueueCapacity = 1000

// VoteAggregator stores the votes and aggregates them into a QC when enough votes have been collected
// VoteAggregator is designed in a way that it can aggregate votes for collection & consensus clusters
// that is why implementation relies on dependency injection.
type VoteAggregator struct {
	*component.ComponentManager
	log                        zerolog.Logger
	hotstuffMetrics            module.HotstuffMetrics
	engineMetrics              module.EngineMetrics
	notifier                   hotstuff.VoteAggregationViolationConsumer
	lowestRetainedView         counters.StrictMonotonousCounter // lowest view, for which we still process votes
	collectors                 hotstuff.VoteCollectors
	queuedMessagesNotifier     engine.Notifier
	finalizationEventsNotifier engine.Notifier
	finalizedView              counters.StrictMonotonousCounter // cache the last finalized view to queue up the pruning work, and unblock the caller who's delivering the finalization event.
	queuedVotes                *fifoqueue.FifoQueue
	queuedBlocks               *fifoqueue.FifoQueue
}

var _ hotstuff.VoteAggregator = (*VoteAggregator)(nil)
var _ component.Component = (*VoteAggregator)(nil)

// NewVoteAggregator creates an instance of vote aggregator
// Note: verifyingProcessorFactory is injected. Thereby, the code is agnostic to the
// different voting formats of main Consensus vs Collector consensus.
func NewVoteAggregator(
	log zerolog.Logger,
	hotstuffMetrics module.HotstuffMetrics,
	engineMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	notifier hotstuff.VoteAggregationViolationConsumer,
	lowestRetainedView uint64,
	collectors hotstuff.VoteCollectors,
) (*VoteAggregator, error) {

	queuedVotes, err := fifoqueue.NewFifoQueue(defaultVoteQueueCapacity,
		fifoqueue.WithLengthObserver(func(len int) { mempoolMetrics.MempoolEntries(metrics.ResourceBlockVoteQueue, uint(len)) }))
	if err != nil {
		return nil, fmt.Errorf("could not initialize votes queue")
	}

	queuedBlocks, err := fifoqueue.NewFifoQueue(defaultBlockQueueCapacity) // TODO metrics
	if err != nil {
		return nil, fmt.Errorf("could not initialize blocks queue")
	}

	aggregator := &VoteAggregator{
		log:                        log.With().Str("component", "hotstuff.vote_aggregator").Logger(),
		hotstuffMetrics:            hotstuffMetrics,
		engineMetrics:              engineMetrics,
		notifier:                   notifier,
		lowestRetainedView:         counters.NewMonotonousCounter(lowestRetainedView),
		finalizedView:              counters.NewMonotonousCounter(lowestRetainedView),
		collectors:                 collectors,
		queuedVotes:                queuedVotes,
		queuedBlocks:               queuedBlocks,
		queuedMessagesNotifier:     engine.NewNotifier(),
		finalizationEventsNotifier: engine.NewNotifier(),
	}

	// manager for own worker routines plus the internal collectors
	componentBuilder := component.NewComponentManagerBuilder()
	var wg sync.WaitGroup
	wg.Add(defaultVoteAggregatorWorkers)
	for i := 0; i < defaultVoteAggregatorWorkers; i++ { // manager for worker routines that process inbound messages
		componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			defer wg.Done()
			ready()
			aggregator.queuedMessagesProcessingLoop(ctx)
		})
	}
	componentBuilder.AddWorker(func(_ irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// create new context which is not connected to parent
		// we need to ensure that our internal workers stop before asking
		// vote collectors to stop. We want to avoid delivering events to already stopped vote collectors
		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx, _ := irrecoverable.WithSignaler(ctx)
		// start vote collectors
		collectors.Start(signalerCtx)
		<-collectors.Ready()

		ready()

		// wait for internal workers to stop
		wg.Wait()
		// signal vote collectors to stop
		cancel()
		// wait for it to stop
		<-collectors.Done()
	})
	componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		aggregator.finalizationProcessingLoop(ctx)
	})

	aggregator.ComponentManager = componentBuilder.Build()
	return aggregator, nil
}

func (va *VoteAggregator) queuedMessagesProcessingLoop(ctx irrecoverable.SignalerContext) {
	notifier := va.queuedMessagesNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := va.processQueuedMessages(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued messages: %w", err))
				return
			}
		}
	}
}

// processQueuedMessages is a function which dispatches previously queued messages on worker thread
// This function is called whenever we have queued messages ready to be dispatched.
// No errors are expected during normal operations.
func (va *VoteAggregator) processQueuedMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := va.queuedBlocks.Pop()
		if ok {
			block := msg.(*model.Proposal)
			err := va.processQueuedBlock(block)
			if err != nil {
				return fmt.Errorf("could not process pending block %v: %w", block.Block.BlockID, err)
			}

			continue
		}

		msg, ok = va.queuedVotes.Pop()
		if ok {
			vote := msg.(*model.Vote)
			startTime := time.Now()
			err := va.processQueuedVote(vote)
			// report duration of processing one vote
			va.hotstuffMetrics.VoteProcessingDuration(time.Since(startTime))
			va.engineMetrics.MessageHandled(metrics.EngineVoteAggregator, metrics.MessageBlockVote)

			if err != nil {
				return fmt.Errorf("could not process pending vote %v for block %v: %w", vote.ID(), vote.BlockID, err)
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
	collector, created, err := va.collectors.GetOrCreateCollector(vote.View)
	if err != nil {
		// ignore if our routine is outdated and some other one has pruned collectors
		if mempool.IsBelowPrunedThresholdError(err) {
			return nil
		}
		return fmt.Errorf("could not get collector for view %d: %w",
			vote.View, err)
	}
	if created {
		va.log.Info().Uint64("view", vote.View).Msg("vote collector is created by processing vote")
	}

	err = collector.AddVote(vote)
	if err != nil {
		return fmt.Errorf("could not process vote for view %d, blockID %v: %w",
			vote.View, vote.BlockID, err)
	}

	va.log.Info().
		Uint64("view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Str("vote_id", vote.ID().String()).
		Msg("vote has been processed successfully")

	return nil
}

// processQueuedBlock performs actual processing of queued block proposals, this method is called from multiple
// concurrent goroutines.
// CAUTION: we expect that the input block's validity has been confirmed prior to calling AddBlock,
// including the proposer's signature. Otherwise, VoteAggregator might crash or exhibit undefined
// behaviour.
// No errors are expected during normal operation.
func (va *VoteAggregator) processQueuedBlock(block *model.Proposal) error {
	// check if the block is for a view that has already been pruned (and is thus stale)
	if block.Block.View < va.lowestRetainedView.Value() {
		return nil
	}

	collector, created, err := va.collectors.GetOrCreateCollector(block.Block.View)
	if err != nil {
		if mempool.IsBelowPrunedThresholdError(err) {
			return nil
		}
		return fmt.Errorf("could not get or create collector for block %v: %w", block.Block.BlockID, err)
	}
	if created {
		va.log.Info().
			Uint64("view", block.Block.View).
			Hex("block_id", block.Block.BlockID[:]).
			Msg("vote collector is created by processing block")
	}

	err = collector.ProcessBlock(block)
	if err != nil {
		if model.IsInvalidProposalError(err) {
			// We are attempting process a block which is invalid
			// This should never happen, because any component that feeds blocks into VoteAggregator
			// needs to make sure that it's submitting for processing ONLY valid blocks.
			return fmt.Errorf("received invalid block for processing %v at view %d", block.Block.BlockID, block.Block.View)
		}
		return fmt.Errorf("could not process block: %v, %w", block.Block.BlockID, err)
	}

	va.log.Info().
		Uint64("view", block.Block.View).
		Hex("block_id", block.Block.BlockID[:]).
		Msg("block has been processed successfully")

	return nil
}

// AddVote checks if vote is stale and appends vote into processing queue
// actual vote processing will be called in other dispatching goroutine.
func (va *VoteAggregator) AddVote(vote *model.Vote) {
	log := va.log.With().Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("voter", vote.SignerID[:]).
		Str("vote_id", vote.ID().String()).Logger()
	// drop stale votes
	if vote.View < va.lowestRetainedView.Value() {
		log.Debug().Msg("drop stale votes")
		va.engineMetrics.InboundMessageDropped(metrics.EngineVoteAggregator, metrics.MessageBlockVote)
		return
	}

	// It's ok to silently drop votes in case our processing pipeline is full.
	// It means that we are probably catching up.
	if ok := va.queuedVotes.Push(vote); ok {
		va.queuedMessagesNotifier.Notify()
	} else {
		log.Info().Msg("no queue capacity, dropping vote")
		va.engineMetrics.InboundMessageDropped(metrics.EngineVoteAggregator, metrics.MessageBlockVote)
	}
}

// AddBlock notifies the VoteAggregator that it should start processing votes for the given block.
// The input block is queued internally within the `VoteAggregator` and processed _asynchronously_
// by the VoteAggregator's internal worker routines.
// CAUTION: we expect that the input block's validity has been confirmed prior to calling AddBlock,
// including the proposer's signature. Otherwise, VoteAggregator might crash or exhibit undefined
// behaviour.
func (va *VoteAggregator) AddBlock(block *model.Proposal) {
	// It's ok to silently drop blocks in case our processing pipeline is full.
	// It means that we are probably catching up.
	if ok := va.queuedBlocks.Push(block); ok {
		va.queuedMessagesNotifier.Notify()
	} else {
		va.log.Debug().Msgf("dropping block %x because queue is full", block.Block.BlockID)
	}
}

// InvalidBlock notifies the VoteAggregator about an invalid proposal, so that it
// can process votes for the invalid block and slash the voters.
// No errors are expected during normal operations
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
		if mempool.IsBelowPrunedThresholdError(err) {
			return nil
		}
		return fmt.Errorf("could not retrieve vote collector for view %d: %w", block.View, err)
	}
	// registering vote consumer will deliver all previously cached votes in strict order
	// and will keep delivering votes if more are collected
	collector.RegisterVoteConsumer(slashingVoteConsumer)
	return nil
}

// PruneUpToView deletes all votes _below_ to the given view, as well as
// related indices. We only retain and process whose view is equal or larger
// than `lowestRetainedView`. If `lowestRetainedView` is smaller than the
// previous value, the previous value is kept and the method call is a NoOp.
func (va *VoteAggregator) PruneUpToView(lowestRetainedView uint64) {
	if va.lowestRetainedView.Set(lowestRetainedView) {
		va.collectors.PruneUpToView(lowestRetainedView)
	}
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs sealing.Core about finalization of respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (va *VoteAggregator) OnFinalizedBlock(block *model.Block) {
	if va.finalizedView.Set(block.View) {
		va.finalizationEventsNotifier.Notify()
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (va *VoteAggregator) finalizationProcessingLoop(ctx context.Context) {
	finalizationNotifier := va.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finalizationNotifier:
			va.PruneUpToView(va.finalizedView.Value())
		}
	}
}
