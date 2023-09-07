package timeoutcollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/counters"
)

// TimeoutCollector implements logic for collecting timeout objects. Performs deduplication, caching and processing
// of timeouts, delegating those tasks to underlying modules. Emits notifications about verified QCs and TCs, if
// their view is newer than any QC or TC previously known to the TimeoutCollector.
// This module is safe to use in concurrent environment.
type TimeoutCollector struct {
	log              zerolog.Logger
	timeoutsCache    *TimeoutObjectsCache // cache for tracking double timeout and timeout equivocation
	notifier         hotstuff.TimeoutAggregationConsumer
	processor        hotstuff.TimeoutProcessor
	newestReportedQC counters.StrictMonotonousCounter // view of newest QC that was reported
	newestReportedTC counters.StrictMonotonousCounter // view of newest TC that was reported
}

var _ hotstuff.TimeoutCollector = (*TimeoutCollector)(nil)

// NewTimeoutCollector creates new instance of TimeoutCollector
func NewTimeoutCollector(log zerolog.Logger,
	view uint64,
	notifier hotstuff.TimeoutAggregationConsumer,
	processor hotstuff.TimeoutProcessor,
) *TimeoutCollector {
	return &TimeoutCollector{
		log: log.With().
			Str("component", "hotstuff.timeout_collector").
			Uint64("view", view).
			Logger(),
		notifier:         notifier,
		timeoutsCache:    NewTimeoutObjectsCache(view),
		processor:        processor,
		newestReportedQC: counters.NewMonotonousCounter(0),
		newestReportedTC: counters.NewMonotonousCounter(0),
	}
}

// AddTimeout adds a Timeout Object [TO] to the collector.
// When TOs from strictly more than 1/3 of consensus participants (measured by weight)
// were collected, the callback for partial TC will be triggered.
// After collecting TOs from a supermajority, a TC will be created and passed to the EventLoop.
// Expected error returns during normal operations:
//   - timeoutcollector.ErrTimeoutForIncompatibleView - submitted timeout for incompatible view
//
// All other exceptions are symptoms of potential state corruption.
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
		if invalidTimeoutErr, ok := model.AsInvalidTimeoutError(err); ok {
			c.notifier.OnInvalidTimeoutDetected(*invalidTimeoutErr)
			return nil
		}
		return fmt.Errorf("internal error while processing timeout: %w", err)
	}

	// TODO: consider moving OnTimeoutProcessed to TimeoutAggregationConsumer, need to fix telemetry for this.
	c.notifier.OnTimeoutProcessed(timeout)

	// In the following, we emit notifications about new QCs, if their view is newer than any QC previously
	// known to the TimeoutCollector. Note that our implementation only provides weak ordering:
	//  * Over larger time scales, the emitted events are for statistically increasing views.
	//  * However, on short time scales there are _no_ monotonicity guarantees w.r.t. the views.
	// Explanation:
	// While only QCs with strict monotonously increasing views pass the
	// `if c.newestReportedQC.Set(timeout.NewestQC.View)` statement, we emit the notification in a separate
	// step. Therefore, emitting the notifications is subject to races, where on very short time-scales
	// the notifications can be out of order.
	// Nevertheless, we note that notifications are only created for QCs that are strictly newer than any other
	// known QC at the time we check via the `if ... Set(..)` statement. Thereby, we implement the desired filtering
	// behaviour, i.e. that the recipient of the notifications is not spammed by old (or repeated) QCs.
	// Reasoning for this approach:
	// The current implementation is completely lock-free without noteworthy risk of congestion. For the recipient
	// of the notifications, the weak ordering is of no concern, because it anyway is only interested in the newest
	// QC. Time-localized disorder is irrelevant, because newer QCs that would arrive later in a strongly ordered
	// system can only arrive earlier in our weakly ordered implementation. Hence, if anything, the recipient
	// receives the desired information _earlier_ but not later.
	if c.newestReportedQC.Set(timeout.NewestQC.View) {
		c.notifier.OnNewQcDiscovered(timeout.NewestQC)
	}
	// Same explanation for weak ordering of QCs also applies to TCs.
	if timeout.LastViewTC != nil {
		if c.newestReportedTC.Set(timeout.LastViewTC.View) {
			c.notifier.OnNewTcDiscovered(timeout.LastViewTC)
		}
	}

	return nil
}

// View returns view which is associated with this timeout collector
func (c *TimeoutCollector) View() uint64 {
	return c.timeoutsCache.View()
}
