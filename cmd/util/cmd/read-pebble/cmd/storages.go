package cmd

import (
	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
)

func InitStorages() (*storage.All, *pebble.DB) {
	db, err := common.InitStoragePebble(flagDatadir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize storage")
	}
	storages := common.InitStoragesPebble(db)
	return storages, db
}
