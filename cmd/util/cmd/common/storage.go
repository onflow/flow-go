package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func InitStorage(datadir string) *badger.DB {
	opts := badger.
		DefaultOptions(datadir).
		WithKeepL0InMemory(true).
		WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("could not open key-value store")
	}

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err = operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize max tracker")
	}

	return db
}
