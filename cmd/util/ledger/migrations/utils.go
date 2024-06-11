package migrations

import (
	"errors"
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func getDomainPayloads(payloads []*ledger.Payload) (map[flow.RegisterID]*ledger.Payload, error) {
	domains := make(map[flow.RegisterID]*ledger.Payload)

	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		if registerID.IsInternalState() || registerID.IsSlabIndex() {
			continue
		}

		domains[registerID] = payload
	}

	return domains, nil
}

func CheckDomainPayloads(payloads []*ledger.Payload) error {
	domainPayloads, err := getDomainPayloads(payloads)
	if err != nil {
		return err
	}

	if len(domainPayloads) == 0 {
		return nil
	}

	var errs []error

	slabIndexLength := len(atree.SlabIndex{})

	for id, p := range domainPayloads {
		if _, exist := allStorageMapDomainsSet[id.Key]; !exist {
			errs = append(errs, fmt.Errorf("found unexpected domain: %s", id.Key))
			continue
		}
		v := p.Value()
		if len(v) != slabIndexLength {
			errs = append(errs, fmt.Errorf("domain payload contains unexpected value: %s %x", id.Key, v))
		}
	}

	return errors.Join(errs...)
}

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

var allStorageMapDomains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
	stdlib.InboxStorageDomain,
	stdlib.CapabilityControllerStorageDomain,
	stdlib.PathCapabilityStorageDomain,
	stdlib.AccountCapabilityStorageDomain,
}

var allStorageMapDomainsSet = map[string]struct{}{}

func init() {
	for _, domain := range allStorageMapDomains {
		allStorageMapDomainsSet[domain] = struct{}{}
	}
}
