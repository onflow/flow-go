package cmd

import "github.com/onflow/flow-go/cmd/util/cmd/common"

func InitStorages() *common.Storages {
	db := common.InitStorage(flagDatadir)
	storages := common.InitStorages(db)
	return storages
}
