package timeoutaggregator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
)

// defaultTimeoutAggregatorWorkers number of workers to dispatch events for timeout aggregator
const defaultTimeoutAggregatorWorkers = 4

// defaultTimeoutQueueCapacity maximum capacity for buffering unprocessed timeouts
const defaultTimeoutQueueCapacity = 1000

// TimeoutAggregator stores the timeout objects and aggregates them into a TC when enough TOs have been collected.
// It's safe to use in concurrent environment.
type TimeoutAggregator struct {
	*component.ComponentManager
	notifications.NoopConsumer
	log                    zerolog.Logger
	hotstuffMetrics        module.HotstuffMetrics
	engineMetrics          module.EngineMetrics
	lowestRetainedView     counters.StrictMonotonousCounter // lowest view, for which we still process timeouts
	collectors             hotstuff.TimeoutCollectors
	queuedTimeoutsNotifier engine.Notifier
	enteringViewNotifier   engine.Notifier
	queuedTimeouts         *fifoqueue.FifoQueue
}

var _ hotstuff.TimeoutAggregator = (*TimeoutAggregator)(nil)
var _ component.Component = (*TimeoutAggregator)(nil)

// NewTimeoutAggregator creates an instance of timeout aggregator.
// No errors are expected during normal operations.
func NewTimeoutAggregator(log zerolog.Logger,
	hotstuffMetrics module.HotstuffMetrics,
	engineMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	lowestRetainedView uint64,
	collectors hotstuff.TimeoutCollectors,
) (*TimeoutAggregator, error) {
	queuedTimeouts, err := fifoqueue.NewFifoQueue(defaultTimeoutQueueCapacity,
		fifoqueue.WithLengthObserver(func(len int) { mempoolMetrics.MempoolEntries(metrics.ResourceTimeoutObjectQueue, uint(len)) }))
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeouts queue")
	}

	aggregator := &TimeoutAggregator{
		log:                    log.With().Str("component", "hotstuff.timeout_aggregator").Logger(),
		hotstuffMetrics:        hotstuffMetrics,
		engineMetrics:          engineMetrics,
		lowestRetainedView:     counters.NewMonotonousCounter(lowestRetainedView),
		collectors:             collectors,
		queuedTimeoutsNotifier: engine.NewNotifier(),
		enteringViewNotifier:   engine.NewNotifier(),
		queuedTimeouts:         queuedTimeouts,
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
			err := t.processQueuedTimeoutObjects(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued timeout events: %w", err))
				return
			}
		}
	}
}

// processQueuedTimeoutObjects sequentially processes items from `queuedTimeouts`
// until the queue returns 'empty'. Only when there are no more queued up TimeoutObjects,
// this function call returns.
// No errors are expected during normal operations.
func (t *TimeoutAggregator) processQueuedTimeoutObjects(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := t.queuedTimeouts.Pop()
		if !ok {
			// when there is no more messages in the queue, back to the loop to wait
			// for the next incoming message to arrive.
			return nil
		}

		timeoutObject := msg.(*model.TimeoutObject)
		startTime := time.Now()

		err := t.processQueuedTimeout(timeoutObject)
		// report duration of processing one timeout object
		t.hotstuffMetrics.TimeoutObjectProcessingDuration(time.Since(startTime))
		t.engineMetrics.MessageHandled(metrics.EngineTimeoutAggregator, metrics.MessageTimeoutObject)

		if err != nil {
			return fmt.Errorf("could not process pending TO %v: %w", timeoutObject.ID(), err)
		}

		t.log.Info().
			Uint64("view", timeoutObject.View).
			Hex("signer", timeoutObject.SignerID[:]).
			Msg("TimeoutObject processed successfully")
	}
}

// processQueuedTimeout performs actual processing of queued timeouts, this method is called from multiple
// concurrent goroutines.
// No errors are expected during normal operation
func (t *TimeoutAggregator) processQueuedTimeout(timeoutObject *model.TimeoutObject) error {
	// We create a timeout collector before validating the first TO, so processing an invalid TO will
	// result in a collector being added, until the corresponding view is pruned.
	// Since epochs on mainnet have 500,000-1,000,000 views, there is an opportunity for memory exhaustion here.
	// However, because only invalid TOs can result in creating collectors at an arbitrary view,
	// and invalid TOs are slashable, resulting eventually in ejection of the sender, this attack is impractical.
	collector, _, err := t.collectors.GetOrCreateCollector(timeoutObject.View)
	if err != nil {
		// ignore if our routine is outdated and some other one has pruned collectors
		if mempool.IsBelowPrunedThresholdError(err) {
			return nil
		}
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			// ignore TO if we don't have information for epoch
			t.log.Debug().Uint64("view", timeoutObject.View).Msg("discarding TO for view beyond known epochs")
			return nil
		}
		return fmt.Errorf("could not get collector for view %d: %w",
			timeoutObject.View, err)
	}

	err = collector.AddTimeout(timeoutObject)
	if err != nil {
		return fmt.Errorf("could not process TO for view %d: %w",
			timeoutObject.View, err)
	}
	return nil
}

// AddTimeout checks if TO is stale and appends TO to processing queue.
// The actual processing will be done asynchronously by the `TimeoutAggregator`'s internal worker routines.
func (t *TimeoutAggregator) AddTimeout(timeoutObject *model.TimeoutObject) {
	// drop stale objects
	if timeoutObject.View < t.lowestRetainedView.Value() {
		t.log.Debug().
			Uint64("view", timeoutObject.View).
			Hex("signer", timeoutObject.SignerID[:]).
			Msg("drop stale timeouts")
		t.engineMetrics.InboundMessageDropped(metrics.EngineTimeoutAggregator, metrics.MessageTimeoutObject)
		return
	}

	placedInQueue := t.queuedTimeouts.Push(timeoutObject)
	if !placedInQueue { // processing pipeline `queuedTimeouts` is full
		// It's ok to silently drop timeouts, because we are probably catching up.
		t.log.Info().
			Uint64("view", timeoutObject.View).
			Hex("signer", timeoutObject.SignerID[:]).
			Msg("no queue capacity, dropping timeout")
		t.engineMetrics.InboundMessageDropped(metrics.EngineTimeoutAggregator, metrics.MessageTimeoutObject)
		return
	}
	t.queuedTimeoutsNotifier.Notify()
}

// PruneUpToView deletes all `TimeoutCollector`s _below_ to the given view, as well as
// related indices. We only retain and process `TimeoutCollector`s, whose view is equal or larger
// than `lowestRetainedView`. If `lowestRetainedView` is smaller than the
// previous value, the previous value is kept and the method call is a NoOp.
func (t *TimeoutAggregator) PruneUpToView(lowestRetainedView uint64) {
	t.collectors.PruneUpToView(lowestRetainedView)
}

// OnViewChange implements the `OnViewChange` callback from the `hotstuff.Consumer`.
// We notify the enteringViewProcessingLoop worker, which then prunes up to the active view.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (t *TimeoutAggregator) OnViewChange(_, newView uint64) {
	if t.lowestRetainedView.Set(newView) {
		t.enteringViewNotifier.Notify()
	}
}

// enteringViewProcessingLoop is a separate goroutine that performs processing of entering view events
func (t *TimeoutAggregator) enteringViewProcessingLoop(ctx context.Context) {
	notifier := t.enteringViewNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			t.PruneUpToView(t.lowestRetainedView.Value())
		}
	}
}
