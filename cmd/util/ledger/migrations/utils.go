package migrations

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

type RegistersMigration func(registersByAccount *registers.ByAccount) error

var allStorageMapDomains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
	stdlib.InboxStorageDomain,
	stdlib.CapabilityControllerStorageDomain,
}

var allStorageMapDomainsSet = map[string]struct{}{}

func init() {
	for _, domain := range allStorageMapDomains {
		allStorageMapDomainsSet[domain] = struct{}{}
	}
}

func getSlabIDsFromRegisters(registers registers.Registers) ([]atree.StorageID, error) {
	storageIDs := make([]atree.StorageID, 0, registers.Count())

	err := registers.ForEach(func(owner string, key string, value []byte) error {

		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		storageID := atree.NewStorageID(
			atree.Address([]byte(owner)),
			atree.StorageIndex([]byte(key[1:])),
		)

		storageIDs = append(storageIDs, storageID)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return storageIDs, nil
}

func loadAtreeSlabsInStorage(
	storage *runtime.Storage,
	registers registers.Registers,
	nWorkers int,
) error {

	storageIDs, err := getSlabIDsFromRegisters(registers)
	if err != nil {
		return err
	}

	return storage.PersistentSlabStorage.BatchPreload(storageIDs, nWorkers)
}

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	registers registers.Registers,
	nWorkers int,
) error {

	err := loadAtreeSlabsInStorage(storage, registers, nWorkers)
	if err != nil {
		return err
	}

	for _, domain := range allStorageMapDomains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}
