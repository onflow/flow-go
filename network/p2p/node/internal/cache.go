package internal

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
)

// DisallowListCache is the disallow-list cache. It is used to keep track of the disallow-listed peers and the reasons for it.
// Stored disallow-list causes are keyed by the hash of the peerID.
type DisallowListCache struct {
	c *stdmap.Backend[flow.Identifier, map[network.DisallowListedCause]struct{}]
}

// NewDisallowListCache creates a new disallow-list cache. The cache is backed by a stdmap.Backend.
// Args:
// - sizeLimit: the size limit of the cache, i.e., the maximum number of records that the cache can hold, recommended size is 100 * number of authorized nodes.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// Returns:
// - *DisallowListCache: the created cache.
func NewDisallowListCache(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *DisallowListCache {
	backData := herocache.NewCache[map[network.DisallowListedCause]struct{}](sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		logger.With().Str("mempool", "disallow-list-records").Logger(),
		collector)

	return &DisallowListCache{
		c: stdmap.NewBackend(stdmap.WithMutableBackData[flow.Identifier, map[network.DisallowListedCause]struct{}](backData)),
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
	var causes []network.DisallowListedCause
	// The stored cause-set is mutated in place by DisallowFor and AllowFor under the backend lock,
	// so it must be copied while the lock is held.
	err := d.c.Run(func(backData mempool.BackData[flow.Identifier, map[network.DisallowListedCause]struct{}]) error {
		stored, exists := backData.Get(p2p.MakeId(peerID))
		if exists && len(stored) > 0 {
			causes = maps.Keys(stored)
		}
		return nil
	})
	if err != nil {
		// the callback never returns an error, so this is unreachable
		panic(fmt.Errorf("unexpected error reading disallow-list cache: %w", err))
	}

	return causes, len(causes) > 0
}

// DisallowFor disallow-lists a peer for a cause.
// Args:
// - peerID: the peerID of the peer to be disallow-listed.
// - cause: the cause for disallow-listing the peer.
// Returns:
// - []network.DisallowListedCause: the list of causes for which the peer is disallow-listed.
// - error: if the operation fails, error is irrecoverable.
func (d *DisallowListCache) DisallowFor(peerID peer.ID, cause network.DisallowListedCause) ([]network.DisallowListedCause, error) {
	initLogic := func() map[network.DisallowListedCause]struct{} {
		return make(map[network.DisallowListedCause]struct{})
	}

	// the adjust callback runs under the backend lock, so the copy of the causes (returned to the
	// caller to avoid external mutation) must be made inside of it.
	var updatedCauses []network.DisallowListedCause
	adjustLogic := func(causes map[network.DisallowListedCause]struct{}) map[network.DisallowListedCause]struct{} {
		causes[cause] = struct{}{}
		updatedCauses = maps.Keys(causes)
		return causes
	}
	_, adjusted := d.c.AdjustWithInit(p2p.MakeId(peerID), adjustLogic, initLogic)
	if !adjusted {
		return nil, fmt.Errorf("failed to disallow list peer %s for cause %s", peerID, cause)
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
	// the adjust callback runs under the backend lock, so the copy of the causes (returned to the
	// caller to avoid external mutation) must be made inside of it.
	var updatedCauses []network.DisallowListedCause
	_, adjusted := d.c.Adjust(p2p.MakeId(peerID), func(causes map[network.DisallowListedCause]struct{}) map[network.DisallowListedCause]struct{} {
		delete(causes, cause)
		updatedCauses = maps.Keys(causes)
		return causes
	})

	if !adjusted {
		// if the entity is not found in the cache, we return an empty list.
		// we don't return a nil to be consistent with the case that entity is found but the list of causes is empty.
		return make([]network.DisallowListedCause, 0)
	}

	return updatedCauses
}
