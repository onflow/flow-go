package timeoutcollector

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
)

type TimeoutCollector struct {
	notifier      hotstuff.Consumer
	timeoutsCache *TimeoutObjectsCache // to track double timeout and timeout equivocation
	processor     hotstuff.TimeoutProcessor

	// a callback that will be used to notify PaceMaker
	// about timeout for higher view that we are aware of
	onNewQCDiscovered hotstuff.OnNewQCDiscovered
	onNewTCDiscovered hotstuff.OnNewTCDiscovered
	// highest QC that was reported
	highestReportedQCTimeout counters.StrictMonotonousCounter
	// highest TC that was reported
	highestReportedTCTimeout counters.StrictMonotonousCounter
}

var _ hotstuff.TimeoutCollector = (*TimeoutCollector)(nil)

func (c *TimeoutCollector) AddTimeout(timeout *model.TimeoutObject) error {
	// cache timeout
	err := c.timeoutsCache.AddTimeoutObject(timeout)
	if err != nil {
		if errors.Is(err, ErrRepeatedTimeout) {
			return nil
		}
		if _, isDoubleTimeoutErr := model.AsDoubleTimeoutError(err); isDoubleTimeoutErr {
			// TODO(active-pacemaker): report to notifier
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

func (c *TimeoutCollector) processTimeout(timeout *model.TimeoutObject) error {
	err := c.processor.Process(timeout)
	if err != nil {
		if model.IsInvalidTimeoutError(err) {
			// invalid signature, potentially slashing challenge
			// notify about invalid timeout
			return nil
		}
		return fmt.Errorf("internal error while processing timeout: %w", err)
	}

	if c.highestReportedQCTimeout.Set(timeout.HighestQC.View) {
		c.onNewQCDiscovered(timeout.HighestQC)
	}

	if c.highestReportedTCTimeout.Set(timeout.LastViewTC.View) {
		c.onNewTCDiscovered(timeout.LastViewTC)
	}

	return nil
}

func (c *TimeoutCollector) View() uint64 {
	return c.timeoutsCache.View()
}
