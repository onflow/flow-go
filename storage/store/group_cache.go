package store

import (
	lru "github.com/fxamacker/golang-lru/v2"

	"github.com/onflow/flow-go/module"
)

// GroupCache extends the Cache with a primary index on K and secondary index on G,
// which can be used to remove multiple cached items efficiently.
// A common use case of GroupCache is to cache data by concatenated key
// (block ID and transaction ID) for faster retrieval, and
// to remove cached items by first key (block ID).
// Although G can be a prefix of K since that can be useful,
// G can be anything comparable (it doesn't need to be a prefix).
type GroupCache[G comparable, K comparable, V any] struct {
	Cache[K, V]
}

func newGroupCache[G comparable, K comparable, V any](
	collector module.CacheMetrics,
	resourceName string,
	groupFromKey func(K) G,
	options ...func(*Cache[K, V]),
) (*GroupCache[G, K, V], error) {
	c := Cache[K, V]{
		metrics:       collector,
		limit:         1000,
		store:         noStore[K, V],
		storeWithLock: noStoreWithLock[K, V],
		retrieve:      noRetrieve[K, V],
		remove:        noRemove[K],
		resource:      resourceName,
	}
	for _, option := range options {
		option(&c)
	}
	var err error
	c.cache, err = lru.NewGroupCache[G, K, V](int(c.limit), groupFromKey)
	if err != nil {
		return nil, err
	}
	c.metrics.CacheEntries(c.resource, uint(c.cache.Len()))
	return &GroupCache[G, K, V]{
		Cache: c,
	}, nil
}

// RemoveGroup removes all cached items associated with the given group.
func (c *GroupCache[G, K, V]) RemoveGroup(group G) int {
	return c.cache.(*lru.GroupCache[G, K, V]).RemoveGroup(group)
}

// RemoveGroup removes all cached items associated with the given groups.
// RemoveGroup should be used to remove multiple groups to
// reduce number of times cache is locked.
func (c *GroupCache[G, K, V]) RemoveGroups(groups []G) int {
	return c.cache.(*lru.GroupCache[G, K, V]).RemoveGroups(groups)
}
