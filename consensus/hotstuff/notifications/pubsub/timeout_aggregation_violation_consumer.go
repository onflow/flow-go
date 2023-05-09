package pubsub

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TimeoutAggregationViolationDistributor ingests notifications about HotStuff-protocol violations and
// distributes them to subscribers. Such notifications are produced by the active consensus
// participants and to a lesser degree also the consensus follower.
// Concurrently safe.
type TimeoutAggregationViolationDistributor struct {
	subscribers []hotstuff.TimeoutAggregationViolationConsumer
	lock        sync.RWMutex
}

var _ hotstuff.TimeoutAggregationViolationConsumer = (*TimeoutAggregationViolationDistributor)(nil)

func NewTimeoutAggregationViolationDistributor() *TimeoutAggregationViolationDistributor {
	return &TimeoutAggregationViolationDistributor{}
}

func (d *TimeoutAggregationViolationDistributor) AddTimeoutAggregationViolationConsumer(consumer hotstuff.TimeoutAggregationViolationConsumer) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.subscribers = append(d.subscribers, consumer)
}

func (d *TimeoutAggregationViolationDistributor) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, altTimeout *model.TimeoutObject) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnDoubleTimeoutDetected(timeout, altTimeout)
	}
}

func (d *TimeoutAggregationViolationDistributor) OnInvalidTimeoutDetected(err model.InvalidTimeoutError) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	for _, subscriber := range d.subscribers {
		subscriber.OnInvalidTimeoutDetected(err)
	}
}
