package events

import (
	"github.com/onflow/flow-go/engine/collection"
)

// CollectionEngineEventsDistributor set of structs that implement all collection engine event interfaces.
type CollectionEngineEventsDistributor struct {
	*ClusterEventsDistributor
}

var _ collection.EngineEvents = (*CollectionEngineEventsDistributor)(nil)

// NewDistributor returns a new *CollectionEngineEventsDistributor.
func NewDistributor() *CollectionEngineEventsDistributor {
	return &CollectionEngineEventsDistributor{
		ClusterEventsDistributor: NewClusterEventsDistributor(),
	}
}

func (d *CollectionEngineEventsDistributor) AddConsumer(consumer collection.EngineEvents) {
	d.ClusterEventsDistributor.AddConsumer(consumer)
}
