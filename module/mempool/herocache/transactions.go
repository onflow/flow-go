package herocache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// Transactions implements the transaction memory pool.
// Stored transactions body are keyed by transactions body ID.
type Transactions struct {
	*stdmap.Backend[flow.Identifier, *flow.TransactionBody]
}

// NewTransactions implements a transactions mempool based on hero cache.
func NewTransactions(limit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *Transactions {
	t := &Transactions{
		stdmap.NewBackend(
			stdmap.WithMutableBackData[flow.Identifier, *flow.TransactionBody](
				herocache.NewCache[*flow.TransactionBody](limit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "transactions").Logger(),
					collector,
				),
			),
		),
	}

	return t
}
