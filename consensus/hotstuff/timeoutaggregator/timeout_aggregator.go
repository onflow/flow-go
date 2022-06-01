package timeoutaggregator

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/component"
)

// defaultTimeoutAggregatorWorkers number of workers to dispatch events for timeout aggregators
const defaultTimeoutAggregatorWorkers = 4

// defaultTimeoutQueueCapacity maximum capacity of buffering unprocessed timeouts
const defaultTimeoutQueueCapacity = 1000

type TimeoutAggregator struct {
	*component.ComponentManager
	log                    zerolog.Logger
	notifier               hotstuff.Consumer
	lowestRetainedView     counters.StrictMonotonousCounter // lowest view, for which we still process timeouts
	collectors             hotstuff.TimeoutCollectors
	queuedTimeoutsNotifier engine.Notifier
	queuedTimeouts         *fifoqueue.FifoQueue
}

var _ hotstuff.TimeoutAggregator = (*TimeoutAggregator)(nil)
var _ component.Component = (*TimeoutAggregator)(nil)

// NewTimeoutAggregator creates an instance of timeout aggregator
func NewTimeoutAggregator(log zerolog.Logger,
	notifier hotstuff.Consumer,
	lowestRetainedView uint64,
	collectors hotstuff.TimeoutCollectors,
) (*TimeoutAggregator, error) {
	queuedTimeouts, err := fifoqueue.NewFifoQueue(fifoqueue.WithCapacity(defaultTimeoutQueueCapacity))
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeouts queue")
	}

	aggregator := &TimeoutAggregator{
		log:                    log,
		notifier:               notifier,
		lowestRetainedView:     counters.NewMonotonousCounter(lowestRetainedView),
		collectors:             collectors,
		queuedTimeoutsNotifier: engine.NewNotifier(),
		queuedTimeouts:         queuedTimeouts,
	}

	componentBuilder := component.NewComponentManagerBuilder()

	for i := 0; i < defaultTimeoutAggregatorWorkers; i++ { // manager for worker routines that process inbound events
		componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			aggregator.queuedTimeoutsProcessingLoop(ctx)
		})
	}

	aggregator.ComponentManager = componentBuilder.Build()
	return aggregator, nil
}

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
				Str("timeout_id", timeoutObject.ID().String()).
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
func (t *TimeoutAggregator) processQueuedTimeout(timeoutObject *model.TimeoutObject) error {
	collector, created, err := t.collectors.GetOrCreateCollector(timeoutObject.View)
	if created {
		t.log.Info().Uint64("view", timeoutObject.View).Msg("timeouts collector is created by processing TO")
	}

	if err != nil {
		// ignore if our routine is outdated and some other one has pruned collectors
		if mempool.IsDecreasingPruningHeightError(err) {
			return nil
		}
		return fmt.Errorf("could not get collector for view %d: %w",
			timeoutObject.View, err)
	}
	err = collector.AddTimeout(timeoutObject)
	if err != nil {
		if model.IsDoubleTimeoutError(err) {
			doubleTimeoutErr := err.(model.DoubleTimeoutError)
			t.notifier.OnDoubleTimeoutDetected(doubleTimeoutErr.FirstTimeout, doubleTimeoutErr.ConflictingTimeout)
			return nil
		}

		return fmt.Errorf("could not process TO for view %d: %w",
			timeoutObject.View, err)
	}

	return nil
}

// AddTimeout checks if TO is stale and appends TO into processing queue
// actual processing will be called in other dispatching goroutine.
func (t *TimeoutAggregator) AddTimeout(timeoutObject *model.TimeoutObject) {
	// drop stale objects
	if timeoutObject.View < t.lowestRetainedView.Value() {

		t.log.Info().
			Uint64("view", timeoutObject.View).
			Hex("signer", timeoutObject.SignerID[:]).
			Str("timeout_id", timeoutObject.ID().String()).
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
// related indices. We only retain and process whose view is equal or larger
// than `lowestRetainedView`. If `lowestRetainedView` is smaller than the
// previous value, the previous value is kept and the method call is a NoOp.
func (t *TimeoutAggregator) PruneUpToView(lowestRetainedView uint64) {
	if t.lowestRetainedView.Set(lowestRetainedView) {
		t.collectors.PruneUpToView(lowestRetainedView)
	}
}
