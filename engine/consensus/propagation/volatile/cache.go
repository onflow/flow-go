// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package volatile

import (
	"github.com/dgraph-io/ristretto"
	"github.com/pkg/errors"
)

type Volatile struct {
	cache *ristretto.Cache
}

func New() (*Volatile, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // 10x num items on full cache
		MaxCost:     1e6, // 1 MB total size
		BufferItems: 64,  // recommended value
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize cache")
	}
	v := &Volatile{
		cache: cache,
	}
	return v, nil
}
