package migrations

import (
	"fmt"

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

func loadAtreeSlabsInStorage(storage *runtime.Storage, registers registers.Registers) error {

	return registers.ForEach(func(owner string, key string, value []byte) error {

		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		slabID := atree.NewSlabID(
			atree.Address([]byte(owner)),
			atree.SlabIndex([]byte(key[1:])),
		)

		// Retrieve the slab
		_, _, err := storage.Retrieve(slabID)
		if err != nil {
			return fmt.Errorf("failed to retrieve slab %s: %w", slabID, err)
		}

		return nil
	})
}

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	registers registers.Registers,
) error {

	err := loadAtreeSlabsInStorage(storage, registers)
	if err != nil {
		return err
	}

	for _, domain := range allStorageMapDomains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}
