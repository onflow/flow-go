package migrations

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
)

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	payloads []*ledger.Payload,
) error {

	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return fmt.Errorf("failed to convert payload to register: %w", err)
		}

		if !registerID.IsSlabIndex() {
			continue
		}

		// Convert the register ID to a storage ID.
		slabID := atree.NewStorageID(
			atree.Address([]byte(registerID.Owner)),
			atree.StorageIndex([]byte(registerID.Key[1:])))

		// Retrieve the slab.
		_, _, err = storage.Retrieve(slabID)
		if err != nil {
			return fmt.Errorf("failed to retrieve slab %s: %w", slabID, err)
		}
	}

	for _, domain := range domains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}

// convert all domains
var domains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
	stdlib.InboxStorageDomain,
	stdlib.CapabilityControllerStorageDomain,
}

var domainsLookupMap = map[string]struct{}{
	common.PathDomainStorage.Identifier():    {},
	common.PathDomainPrivate.Identifier():    {},
	common.PathDomainPublic.Identifier():     {},
	runtime.StorageDomainContract:            {},
	stdlib.InboxStorageDomain:                {},
	stdlib.CapabilityControllerStorageDomain: {},
}
