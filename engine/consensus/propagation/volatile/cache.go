// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package volatile

import (
	"github.com/dgraph-io/ristretto"
	"github.com/pkg/errors"
)

type Cache struct {
	cache *ristretto.Cache
}

func NewCache() (*Cache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6, // 1M hashes
		MaxCost:     1e5, // 100k hashes
		BufferItems: 64,  // recommended value
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize cache")
	}
	c := &Cache{
		cache: cache,
	}
	return c, nil
}

func (c *Cache) Close() {
	c.cache.Close()
}
