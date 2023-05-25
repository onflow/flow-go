package internal

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p/middleware"
)

type DisallowListCache struct {
	c      *stdmap.Backend
	logger zerolog.Logger
}

var _ middleware.DisallowListCache = (*DisallowListCache)(nil)

func NewDisallowListCache() DisallowListCache {
	return DisallowListCache{
		c: stdmap.NewBackend(),
	}
}

func (d *DisallowListCache) IsDisallowed(peerID peer.ID) ([]middleware.DisallowListedCause, bool) {
	entity, exists := d.c.ByID(makeId(peerID))
	if !exists {
		return nil, false
	}
	dEntity := mustBeDisallowListEntity(entity)

	// returning a deep copy of causes (to avoid being mutated externally).
	causes := make([]middleware.DisallowListedCause, 0, len(dEntity.causes))
	for cause := range dEntity.causes {
		causes = append(causes, cause)
	}
	return causes, true
}

func (d *DisallowListCache) DisallowFor(peerID peer.ID, cause middleware.DisallowListedCause) error {

}

func (d *DisallowListCache) disallowListFor(peerID peer.ID, cause middleware.DisallowListedCause) ([]middleware.DisallowListedCause, error) {
	var rErr error
	adjustedEntity, adjusted := d.c.Adjust(makeId(peerID), func(entity flow.Entity) flow.Entity {
		dEntity := mustBeDisallowListEntity(entity)
		dEntity.causes[cause] = struct{}{}

		return dEntity
	})

	if rErr != nil {
		return nil, fmt.Errorf("failed to update disallow list cache entity: %w", rErr)
	}

	if !adjusted {
		return ErrDisallowCacheEntityNotFound, nil
	}

	dEntity := mustBeDisallowListEntity(adjustedEntity)
	updatedCauses := make([]middleware.DisallowListedCause, 0, len(dEntity.causes))
	for cause := range dEntity.causes {
		updatedCauses = append(updatedCauses, cause)
	}

	return updatedCauses, nil
}

func (d *DisallowListCache) AllowFor(peerID peer.ID, cause middleware.DisallowListedCause) error {
	//TODO implement me
	panic("implement me")
}

// mustBeDisallowListEntity is a helper function for type assertion of the flow.Entity to disallowListCacheEntity.
// It panics if the type assertion fails.
// Args:
// - entity: the flow.Entity to be type asserted.
// Returns:
// - the disallowListCacheEntity.
func mustBeDisallowListEntity(entity flow.Entity) *disallowListCacheEntity {
	dEntity, ok := entity.(*disallowListCacheEntity)
	if !ok {
		// this should never happen, unless there is a bug. We should crash the node and do not proceed.
		panic(fmt.Errorf("disallow list cache entity is not of type disallowListCacheEntity, got: %T", entity))
	}
	return dEntity
}
