package cache

type OnEntityEjected[V any] func(ejectedEntity V)

// HeroCacheDistributor implements herocache.Tracer and allows subscribers to receive events
// for ejected entries from cache using herocache.Tracer API.
// This structure is NOT concurrency safe.
type HeroCacheDistributor[V any] struct {
	consumers []OnEntityEjected[V]
}

func NewDistributor[V any]() *HeroCacheDistributor[V] {
	return &HeroCacheDistributor[V]{}
}

// AddConsumer adds subscriber for entity ejected events.
// Is NOT concurrency safe.
func (d *HeroCacheDistributor[V]) AddConsumer(consumer OnEntityEjected[V]) {
	d.consumers = append(d.consumers, consumer)
}

func (d *HeroCacheDistributor[V]) EntityEjectionDueToEmergency(ejectedEntity V) {
	for _, consumer := range d.consumers {
		consumer(ejectedEntity)
	}
}

func (d *HeroCacheDistributor[V]) EntityEjectionDueToFullCapacity(ejectedEntity V) {
	for _, consumer := range d.consumers {
		consumer(ejectedEntity)
	}
}
