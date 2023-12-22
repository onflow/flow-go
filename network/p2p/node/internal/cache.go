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
		// this cache is supposed to keep the disallow-list causes for the authorized (staked) nodes. Since the number of such nodes is
		// expected to be small, we do not eject any records from the cache. The cache size must be large enough to hold all
		// the spam records of the authorized nodes. Also, this cache is keeping at most one record per peer id, so the
		// size of the cache must be at least the number of authorized nodes.
		heropool.NoEjection,
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

// init initializes the disallow-list cache entity for the peerID.
// Args:
// - peerID: the peerID of the peer to be disallow-listed.
// Returns:
//   - bool: true if the entity is successfully added to the cache.
//     false if the entity already exists in the cache.
func (d *DisallowListCache) init(peerID peer.ID) bool {
	return d.c.Add(&disallowListCacheEntity{
		peerID: peerID,
		causes: make(map[network.DisallowListedCause]struct{}),
		id:     makeId(peerID),
	})
}

// DisallowFor disallow-lists a peer for a cause.
// Args:
// - peerID: the peerID of the peer to be disallow-listed.
// - cause: the cause for disallow-listing the peer.
// Returns:
// - []network.DisallowListedCause: the list of causes for which the peer is disallow-listed.
// - error: if the operation fails, error is irrecoverable.
func (d *DisallowListCache) DisallowFor(peerID peer.ID, cause network.DisallowListedCause) ([]network.DisallowListedCause, error) {
	// first, we try to optimistically add the peer to the disallow list.
	causes, err := d.disallowListFor(peerID, cause)
	switch {
	case err == nil:
		return causes, nil
	case err == ErrDisallowCacheEntityNotFound:
		// if the entity not exist, we initialize it and try again.
		// Note: there is an edge case where the entity is initialized by another goroutine between the two calls.
		// In this case, the init function is invoked twice, but it is not a problem because the underlying
		// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
		// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
		// we ignore the return value of the init function.
		_ = d.init(peerID)
		causes, err = d.disallowListFor(peerID, cause)
		if err != nil {
			// any error after the init is irrecoverable.
			return nil, fmt.Errorf("failed to disallow list peer %s for cause %s: %w", peerID, cause, err)
		}
		return causes, nil
	default:
		return nil, fmt.Errorf("failed to disallow list peer %s for cause %s: %w", peerID, cause, err)
	}
}

// disallowListFor is a helper function for disallowing a peer for a cause.
// It adds the cause to the disallow list cache entity for the peerID and returns the updated list of causes for the peer.
// Args:
// - peerID: the peerID of the peer to be disallow-listed.
// - cause: the cause for disallow-listing the peer.
// Returns:
// - the updated list of causes for the peer.
// - error if the entity for the peerID is not found in the cache it returns ErrDisallowCacheEntityNotFound, which is a benign error.
func (d *DisallowListCache) disallowListFor(peerID peer.ID, cause network.DisallowListedCause) ([]network.DisallowListedCause, error) {
	adjustedEntity, adjusted := d.c.Adjust(makeId(peerID), func(entity flow.Entity) flow.Entity {
		dEntity := mustBeDisallowListEntity(entity)
		dEntity.causes[cause] = struct{}{}
		return dEntity
	})

	if !adjusted {
		// if the entity is not found in the cache, we return a benign error.
		return nil, ErrDisallowCacheEntityNotFound
	}

	dEntity := mustBeDisallowListEntity(adjustedEntity)
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
