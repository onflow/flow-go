package migrations

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
)

func getSlabIDsFromPayloads(payloads []*ledger.Payload) ([]atree.SlabID, error) {
	slabIDs := make([]atree.SlabID, 0, len(payloads))

	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		if !registerID.IsSlabIndex() {
			continue
		}

		// Convert the register ID to a storage ID.
		slabID := atree.NewSlabID(
			atree.Address([]byte(registerID.Owner)),
			atree.SlabIndex([]byte(registerID.Key[1:])))

		slabIDs = append(slabIDs, slabID)
	}

	return slabIDs, nil
}

func loadAtreeSlabsInStorge(storage *runtime.Storage, payloads []*ledger.Payload, nWorkers int) error {
	slabIDs, err := getSlabIDsFromPayloads(payloads)
	if err != nil {
		return err
	}

	return storage.PersistentSlabStorage.BatchPreload(slabIDs, nWorkers)
}

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	payloads []*ledger.Payload,
	nWorkers int,
	needToPreloadAtreeSlabs bool,
) error {

	if needToPreloadAtreeSlabs {
		err := loadAtreeSlabsInStorge(storage, payloads, nWorkers)
		if err != nil {
			return err
		}
	}

	for _, domain := range allStorageMapDomains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}

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
