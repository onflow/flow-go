package cache

import (
	"github.com/onflow/flow-go/model/flow"
)

// ActiveClusterIdsEntity is an entity that represents the active cluster IDs. This entity is used to leverage
// the herocache cache already in use to track the number of cluster prefixed topics received by a peer. It allows
// consumption of ClusterIdsUpdated protocol events to be non-blocking.
type ActiveClusterIdsEntity struct {
	Identifier       flow.Identifier
	ActiveClusterIds flow.ChainIDList
}

var _ flow.Entity = (*ActiveClusterIdsEntity)(nil)

// NewActiveClusterIdsEntity returns a new ActiveClusterIdsEntity. The flow zero Identifier will be used to store this special
// purpose entity.
func NewActiveClusterIdsEntity(identifier flow.Identifier, clusterIDList flow.ChainIDList) ActiveClusterIdsEntity {
	return ActiveClusterIdsEntity{
		ActiveClusterIds: clusterIDList,
		Identifier:       identifier,
	}
}

// ID returns the origin id of the spam record, which is used as the unique identifier of the entity for maintenance and
// deduplication purposes in the cache.
func (a ActiveClusterIdsEntity) ID() flow.Identifier {
	return a.Identifier
}

// Checksum returns the origin id of the spam record, it does not have any purpose in the cache.
// It is implemented to satisfy the flow.Entity interface.
func (a ActiveClusterIdsEntity) Checksum() flow.Identifier {
	return a.Identifier
}
