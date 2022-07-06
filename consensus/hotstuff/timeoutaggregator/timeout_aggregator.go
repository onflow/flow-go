package timeoutaggregator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
)

// defaultTimeoutAggregatorWorkers number of workers to dispatch events for timeout aggregator
const defaultTimeoutAggregatorWorkers = 4

// defaultTimeoutQueueCapacity maximum capacity for buffering unprocessed timeouts
const defaultTimeoutQueueCapacity = 1000

// TimeoutAggregator stores the timeout objects and aggregates them into a TC when enough TOs have been collected.
// It's safe to use in concurrent environment.
type TimeoutAggregator struct {
	*component.ComponentManager
	log                        zerolog.Logger
	notifier                   hotstuff.Consumer
	committee                  hotstuff.Replicas
	lowestRetainedView         counters.StrictMonotonousCounter // lowest view, for which we still process timeouts
	collectors                 hotstuff.TimeoutCollectors
	queuedTimeoutsNotifier     engine.Notifier
	enteringViewNotifier engine.Notifier
	queuedTimeouts             *fifoqueue.FifoQueue
}

var _ hotstuff.TimeoutAggregator = (*TimeoutAggregator)(nil)
var _ component.Component = (*TimeoutAggregator)(nil)

// NewTimeoutAggregator creates an instance of timeout aggregator.
// No errors are expected during normal operations.
func NewTimeoutAggregator(log zerolog.Logger,
	notifier hotstuff.Consumer,
	lowestRetainedView uint64,
	committee hotstuff.Replicas,
	collectors hotstuff.TimeoutCollectors,
) (*TimeoutAggregator, error) {
	queuedTimeouts, err := fifoqueue.NewFifoQueue(fifoqueue.WithCapacity(defaultTimeoutQueueCapacity))
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeouts queue")
	}

	aggregator := &TimeoutAggregator{
		log:                        log.With().Str("component", "timeout_aggregator").Logger(),
		notifier:                   notifier,
		lowestRetainedView:         counters.NewMonotonousCounter(lowestRetainedView),
		activeView:                 counters.NewMonotonousCounter(lowestRetainedView),
		committee:                  committee,
		collectors:                 collectors,
		queuedTimeoutsNotifier:     engine.NewNotifier(),
		enteringViewEventsNotifier: engine.NewNotifier(),
		queuedTimeouts:             queuedTimeouts,
	}

	componentBuilder := component.NewComponentManagerBuilder()
	for i := 0; i < defaultTimeoutAggregatorWorkers; i++ { // manager for worker routines that process inbound events
		componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			aggregator.queuedTimeoutsProcessingLoop(ctx)
		})
	}
	componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		aggregator.enteringViewProcessingLoop(ctx)
	})

	aggregator.ComponentManager = componentBuilder.Build()
	return aggregator, nil
}

// queuedTimeoutsProcessingLoop is the event loop which waits for notification about pending work
// and as soon as there is some it triggers processing.
// All errors are propagated to SignalerContext
func (t *TimeoutAggregator) queuedTimeoutsProcessingLoop(ctx irrecoverable.SignalerContext) {
	notifier := t.queuedTimeoutsNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := t.processQueuedTimeoutEvents(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued timeout events: %w", err))
				return
			}
		}
	}
}

// processQueuedTimeoutEvents is a function which dispatches previously queued timeouts on worker thread
// This function is called whenever we have queued timeouts ready to be dispatched.
// No errors are expected during normal operations.
func (t *TimeoutAggregator) processQueuedTimeoutEvents(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := t.queuedTimeouts.Pop()
		if ok {
			timeoutObject := msg.(*model.TimeoutObject)
			err := t.processQueuedTimeout(timeoutObject)
			if err != nil {
				return fmt.Errorf("could not process pending TO %v: %w", timeoutObject.ID(), err)
			}

			t.log.Info().
				Uint64("view", timeoutObject.View).
				Hex("signer", timeoutObject.SignerID[:]).
				Msg("TO has been processed successfully")

			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// processQueuedTimeout performs actual processing of queued timeouts, this method is called from multiple
// concurrent goroutines.
// No errors are expected during normal operation
func (t *TimeoutAggregator) processQueuedTimeout(timeoutObject *model.TimeoutObject) error {
	// TODO: replace this check by a specific function to query if there is epoch by view
	_, err := t.committee.LeaderForView(timeoutObject.View)
	if err != nil {
		// ignore TO if we don't have information for epoch
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			t.log.Debug().Uint64("view", timeoutObject.View).Msg("discarding TO for view beyond known epochs")
			return nil
		}
		return fmt.Errorf("unknown error when querying epoch state for view %d: %w", timeoutObject.View, err)
	}

	// We create a timeout collector before validating the first TO, so processing an invalid TO will
	// result in a collector being added, until the corresponding view is pruned.
	// Since epochs on Mainnet have 500,000-1,000,000 views, there is an opportunity for memory exhaustion here.
	// However, because only invalid TOs can result in creating collectors at an arbitrary view,
	// and invalid TOs are slashable, resulting eventually in ejection of the sender, this attack is impactical.
	collector, created, err := t.collectors.GetOrCreateCollector(timeoutObject.View)
	if err != nil {
		// ignore if our routine is outdated and some other one has pruned collectors
		if mempool.IsDecreasingPruningHeightError(err) {
			return nil
		}
		return fmt.Errorf("could not get collector for view %d: %w",
			timeoutObject.View, err)
	}
	if created {
		t.log.Info().Uint64("view", timeoutObject.View).Msg("timeouts collector is created by processing TO")
	}

	err = collector.AddTimeout(timeoutObject)
	if err != nil {
		if doubleTimeoutErr, is := model.AsDoubleTimeoutError(err); is {
			t.notifier.OnDoubleTimeoutDetected(doubleTimeoutErr.FirstTimeout, doubleTimeoutErr.ConflictingTimeout)
			return nil
		}

		return fmt.Errorf("could not process TO for view %d: %w",
			timeoutObject.View, err)
	}

	return nil
}

// AddTimeout checks if TO is stale and appends TO to processing queue.
// The actual processing will be done asynchronously by one of the `TimeoutAggregator` internal worker routines.
func (t *TimeoutAggregator) AddTimeout(timeoutObject *model.TimeoutObject) {
	// drop stale objects
	if timeoutObject.View < t.lowestRetainedView.Value() {
		t.log.Debug().
			Uint64("view", timeoutObject.View).
			Hex("signer", timeoutObject.SignerID[:]).
			Msg("drop stale timeouts")

		return
	}

	// It's ok to silently drop timeouts in case our processing pipeline is full.
	// It means that we are probably catching up.
	if ok := t.queuedTimeouts.Push(timeoutObject); ok {
		t.queuedTimeoutsNotifier.Notify()
	}
}

// PruneUpToView deletes all timeouts _below_ to the given view, as well as
// related indices. We only retain and process `TimeoutCollector`s, whose view is equal or larger
// than `lowestRetainedView`. If `lowestRetainedView` is smaller than the
// previous value, the previous value is kept and the method call is a NoOp.
func (t *TimeoutAggregator) PruneUpToView(lowestRetainedView uint64) {
	if t.lowestRetainedView.Set(lowestRetainedView) {
		t.collectors.PruneUpToView(lowestRetainedView)
	}
}

// OnEnteringView implements the `OnEnteringView` callback from the `hotstuff.FinalizationConsumer`
// We notify the enteringViewProcessingLoop worker, which then prunes up to the active view.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (t *TimeoutAggregator) OnEnteringView(viewNumber uint64, _ flow.Identifier) {
	if t.activeView.Set(viewNumber) {
		t.enteringViewEventsNotifier.Notify()
	}
}

// enteringViewProcessingLoop is a separate goroutine that performs processing of entering view events
func (t *TimeoutAggregator) enteringViewProcessingLoop(ctx context.Context) {
	notifier := t.enteringViewEventsNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			t.PruneUpToView(t.activeView.Value())
		}
	}
}
