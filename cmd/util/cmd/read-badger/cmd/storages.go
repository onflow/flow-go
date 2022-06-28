package cmd

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InitStorages() (*storage.All, *badger.DB) {
	db := common.InitStorage(flagDatadir)
	storages := common.InitStorages(db)
	return storages, db
}

func InitPersister() (*persister.Persister, *badger.DB) {
	db := common.InitStorage(flagDatadir)
	chainID := flow.ChainID("flow-mainnet")
	return persister.New(db, chainID), db
}
