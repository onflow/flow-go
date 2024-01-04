package internal

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network"
)

var (
	ErrDisallowCacheEntityNotFound = errors.New("disallow list cache entity not found")
)

// DisallowListCache is the disallow-list cache. It is used to keep track of the disallow-listed peers and the reasons for it.
type DisallowListCache struct {
	c *stdmap.Backend
}

// NewDisallowListCache creates a new disallow-list cache. The cache is backed by a stdmap.Backend.
// Args:
// - sizeLimit: the size limit of the cache, i.e., the maximum number of records that the cache can hold, recommended size is 100 * number of authorized nodes.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// Returns:
// - *DisallowListCache: the created cache.
func NewDisallowListCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *DisallowListCache {
	backData := herocache.NewCache(sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger.With().Str("mempool", "disallow-list-records").Logger(),
		collector)

	return &DisallowListCache{
		c: stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// IsDisallowListed determines whether the given peer is disallow-listed for any reason.
// Args:
// - peerID: the peer to check.
// Returns:
// - []network.DisallowListedCause: the list of causes for which the given peer is disallow-listed. If the peer is not disallow-listed for any reason,
// a nil slice is returned.
// - bool: true if the peer is disallow-listed for any reason, false otherwise.
func (d *DisallowListCache) IsDisallowListed(peerID peer.ID) ([]network.DisallowListedCause, bool) {
	entity, exists := d.c.ByID(makeId(peerID))
	if !exists {
		return nil, false
	}

	dEntity := mustBeDisallowListEntity(entity)
	if len(dEntity.causes) == 0 {
		return nil, false
	}

	causes := make([]network.DisallowListedCause, 0, len(dEntity.causes))
	for c := range dEntity.causes {
		causes = append(causes, c)
	}
	return causes, true
}

// DisallowFor disallow-lists a peer for a cause.
// Args:
// - peerID: the peerID of the peer to be disallow-listed.
// - cause: the cause for disallow-listing the peer.
// Returns:
// - []network.DisallowListedCause: the list of causes for which the peer is disallow-listed.
// - error: if the operation fails, error is irrecoverable.
func (d *DisallowListCache) DisallowFor(peerID peer.ID, cause network.DisallowListedCause) ([]network.DisallowListedCause, error) {
	initLogic := func() flow.Entity {
		return &disallowListCacheEntity{
			peerID: peerID,
			causes: make(map[network.DisallowListedCause]struct{}),
			id:     makeId(peerID),
		}
	}
	adjustLogic := func(entity flow.Entity) flow.Entity {
		dEntity := mustBeDisallowListEntity(entity)
		dEntity.causes[cause] = struct{}{}
		return dEntity
	}
	adjustedEntity, adjusted := d.c.AdjustWithInit(makeId(peerID), adjustLogic, initLogic)
	if !adjusted {
		return nil, fmt.Errorf("failed to disallow list peer %s for cause %s", peerID, cause)
	}

	dEntity := mustBeDisallowListEntity(adjustedEntity)
	// returning a deep copy of causes (to avoid being mutated externally).
	updatedCauses := make([]network.DisallowListedCause, 0, len(dEntity.causes))
	for c := range dEntity.causes {
		updatedCauses = append(updatedCauses, c)
	}

	return updatedCauses, nil
}

// AllowFor removes a cause from the disallow list cache entity for the peerID.
// Args:
// - peerID: the peerID of the peer to be allow-listed.
// - cause: the cause for allow-listing the peer.
// Returns:
// - the list of causes for which the peer is disallow-listed.
// - error if the entity for the peerID is not found in the cache it returns ErrDisallowCacheEntityNotFound, which is a benign error.
func (d *DisallowListCache) AllowFor(peerID peer.ID, cause network.DisallowListedCause) []network.DisallowListedCause {
	adjustedEntity, adjusted := d.c.Adjust(makeId(peerID), func(entity flow.Entity) flow.Entity {
		dEntity := mustBeDisallowListEntity(entity)
		delete(dEntity.causes, cause)
		return dEntity
	})

	if !adjusted {
		// if the entity is not found in the cache, we return an empty list.
		// we don't return a nil to be consistent with the case that entity is found but the list of causes is empty.
		return make([]network.DisallowListedCause, 0)
	}

	dEntity := mustBeDisallowListEntity(adjustedEntity)
	// returning a deep copy of causes (to avoid being mutated externally).
	causes := make([]network.DisallowListedCause, 0, len(dEntity.causes))
	for c := range dEntity.causes {
		causes = append(causes, c)
	}
	return causes
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
