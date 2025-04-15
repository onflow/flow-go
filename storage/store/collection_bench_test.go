package store_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func BenchmarkIndexCollectionWithBadger(b *testing.B) {
	unittest.RunWithBadgerDB(b, func(bdb *badger.DB) {
		db := badgerimpl.ToDB(bdb)

		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		const transactionCount = 5

		cols := make([]flow.Collection, b.N)
		for i := range b.N {
			cols[i] = unittest.CollectionFixture(transactionCount)
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = collections.Store(&cols[i])
		}
	})
}
