package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TimeoutAggregationViolationDistributor ingests notifications about timeout aggregation violations and
// distributes them to consumers. Such notifications are produced by the timeout aggregation logic.
// Concurrently safe.
type TimeoutAggregationViolationDistributor struct {
	consumers []hotstuff.TimeoutAggregationViolationConsumer
	lock      sync.RWMutex
}

var _ hotstuff.TimeoutAggregationViolationConsumer = (*TimeoutAggregationViolationDistributor)(nil)

func NewTimeoutAggregationViolationDistributor() *TimeoutAggregationViolationDistributor {
	return &TimeoutAggregationViolationDistributor{}
}

func (d *TimeoutAggregationViolationDistributor) AddTimeoutAggregationViolationConsumer(consumer hotstuff.TimeoutAggregationViolationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.consumers = append(d.consumers, consumer)
}

func (d *TimeoutAggregationViolationDistributor) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, altTimeout *model.TimeoutObject) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnDoubleTimeoutDetected(timeout, altTimeout)
	}
}

func (d *TimeoutAggregationViolationDistributor) OnInvalidTimeoutDetected(err model.InvalidTimeoutError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.consumers {
		subscriber.OnInvalidTimeoutDetected(err)
	}
}
