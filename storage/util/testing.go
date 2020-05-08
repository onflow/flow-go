package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"

	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func StorageLayer(t testing.TB, db *badger.DB) (*storage.Headers, *storage.Identities, *storage.Guarantees, *storage.Seals, *storage.Payloads, *storage.Blocks) {
	headers := storage.NewHeaders(db)
	identities := storage.NewIdentities(db)
	guarantees := storage.NewGuarantees(db)
	seals := storage.NewSeals(db)
	payloads := storage.NewPayloads(db, identities, guarantees, seals)
	blocks := storage.NewBlocks(db, headers, payloads)
	return headers, identities, guarantees, seals, payloads, blocks
}

func RunWithStorageLayer(t testing.TB, f func(*badger.DB, *storage.Headers, *storage.Identities, *storage.Guarantees, *storage.Seals, *storage.Payloads, *storage.Blocks)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		headers, identities, guarantees, seals, payloads, blocks := StorageLayer(t, db)
		f(db, headers, identities, guarantees, seals, payloads, blocks)
	})
}
