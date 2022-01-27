package herocache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type DeduplicationLRUCache struct {
	b *stdmap.Backend
}

func NewDeduplicationLRUCache(sizeLimit uint32, logger zerolog.Logger) *DeduplicationLRUCache {
	return &DeduplicationLRUCache{b: stdmap.NewBackendWithBackData(
		herocache.NewCache(sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection,
			logger.With().Str("mempool", "transactions").Logger()))}
}

func (d *DeduplicationLRUCache) Add(e flow.Entity) bool {
	return d.b.Add(e)
}
