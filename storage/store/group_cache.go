package store

import (
	lru "github.com/fxamacker/golang-lru/v2"

	"github.com/onflow/flow-go/module"
)

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

func (c *GroupCache[G, K, V]) RemoveGroup(group G) int {
	return c.cache.(*lru.GroupCache[G, K, V]).RemoveGroup(group)
}

func (c *GroupCache[G, K, V]) RemoveGroups(groups []G) int {
	return c.cache.(*lru.GroupCache[G, K, V]).RemoveGroups(groups)
}
