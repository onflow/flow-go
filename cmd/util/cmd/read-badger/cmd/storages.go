package cmd

import (
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
)

func InitStorages() *storage.All {
	db := common.InitStorage(flagDatadir)
	storages := common.InitStorages(db)
	return storages
}
