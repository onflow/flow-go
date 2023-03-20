package cache

import (
	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
)

type OnEntityEjected func(ejectedEntity flow.Entity)

// HeroCacheDistributor implements herocache.Tracer and allows subscribers to receive events
// for ejected entries from cache using herocache.Tracer API.
// This structure is NOT concurrency safe.
type HeroCacheDistributor struct {
	consumers []OnEntityEjected
}

var _ herocache.Tracer = (*HeroCacheDistributor)(nil)

func NewDistributor() *HeroCacheDistributor {
	return &HeroCacheDistributor{}
}

// AddConsumer adds subscriber for entity ejected events.
// Is NOT concurrency safe.
func (d *HeroCacheDistributor) AddConsumer(consumer OnEntityEjected) {
	d.consumers = append(d.consumers, consumer)
}

func (d *HeroCacheDistributor) EntityEjectionDueToEmergency(ejectedEntity flow.Entity) {
	for _, consumer := range d.consumers {
		consumer(ejectedEntity)
	}
}

func (d *HeroCacheDistributor) EntityEjectionDueToFullCapacity(ejectedEntity flow.Entity) {
	for _, consumer := range d.consumers {
		consumer(ejectedEntity)
	}
}
