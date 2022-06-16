package timeoutcollector

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
)

// TimeoutCollector implements logic for collecting timeout objects. Performs deduplication, caching and processing
// of timeouts, delegating those tasks to underlying modules. Performs notifications about verified QCs and TCs.
// This module is safe to use in concurrent environment.
type TimeoutCollector struct {
	notifier          hotstuff.Consumer
	timeoutsCache     *TimeoutObjectsCache // cache for tracking double timeout and timeout equivocation
	collectorNotifier hotstuff.TimeoutCollectorConsumer
	processor         hotstuff.TimeoutProcessor
	newestReportedQC  counters.StrictMonotonousCounter // newest QC that was reported
	newestReportedTC  counters.StrictMonotonousCounter // newest TC that was reported
}

var _ hotstuff.TimeoutCollector = (*TimeoutCollector)(nil)

// NewTimeoutCollector creates new instance of TimeoutCollector
func NewTimeoutCollector(view uint64,
	notifier hotstuff.Consumer,
	collectorNotifier hotstuff.TimeoutCollectorConsumer,
	processor hotstuff.TimeoutProcessor,
) *TimeoutCollector {
	return &TimeoutCollector{
		notifier:          notifier,
		timeoutsCache:     NewTimeoutObjectsCache(view),
		processor:         processor,
		collectorNotifier: collectorNotifier,
		newestReportedQC:  counters.NewMonotonousCounter(0),
		newestReportedTC:  counters.NewMonotonousCounter(0),
	}
}

// AddTimeout adds a timeout object to the collector
// When f+1 TOs will be collected then callback for partial TC will be triggered,
// after collecting 2f+1 TOs a TC will be created and passed to the EventLoop.
// No errors are expected during normal flow of operations.
func (c *TimeoutCollector) AddTimeout(timeout *model.TimeoutObject) error {
	// cache timeout
	err := c.timeoutsCache.AddTimeoutObject(timeout)
	if err != nil {
		if errors.Is(err, ErrRepeatedTimeout) {
			return nil
		}
		if doubleTimeoutErr, isDoubleTimeoutErr := model.AsDoubleTimeoutError(err); isDoubleTimeoutErr {
			c.notifier.OnDoubleTimeoutDetected(doubleTimeoutErr.FirstTimeout, doubleTimeoutErr.ConflictingTimeout)
			return nil
		}
		return fmt.Errorf("internal error adding timeout %v to cache for view: %d: %w", timeout.ID(), timeout.View, err)
	}

	err = c.processTimeout(timeout)
	if err != nil {
		return fmt.Errorf("internal error processing TO %v for view: %d: %w", timeout.ID(), timeout.View, err)
	}
	return nil
}

// processTimeout delegates TO processing to TimeoutProcessor, handles sentinel errors
// expected errors are handled and reported to notifier. Notifies listeners about validates
// QCs and TCs.
// No errors are expected during normal flow of operations.
func (c *TimeoutCollector) processTimeout(timeout *model.TimeoutObject) error {
	err := c.processor.Process(timeout)
	if err != nil {
		if model.IsInvalidTimeoutError(err) {
			c.notifier.OnInvalidTimeoutDetected(timeout)
			return nil
		}
		return fmt.Errorf("internal error while processing timeout: %w", err)
	}

	if c.newestReportedQC.Set(timeout.NewestQC.View) {
		c.collectorNotifier.OnNewQcDiscovered(timeout.NewestQC)
	}

	if c.newestReportedTC.Set(timeout.LastViewTC.View) {
		c.collectorNotifier.OnNewTcDiscovered(timeout.LastViewTC)
	}

	return nil
}

// View returns view which is associated with this timeout collector
func (c *TimeoutCollector) View() uint64 {
	return c.timeoutsCache.View()
}
