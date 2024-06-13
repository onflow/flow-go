package migrations

import (
	"errors"
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func getDomainPayloads(accountRegisters *registers.AccountRegisters) map[flow.RegisterID][]byte {
	domains := make(map[flow.RegisterID][]byte)

	_ = accountRegisters.ForEach(func(owner string, key string, value []byte) error {
		registerID := flow.RegisterID{Owner: owner, Key: key}

		if registerID.IsInternalState() || registerID.IsSlabIndex() {
			return nil
		}

		domains[registerID] = value
		return nil
	})

	return domains
}

func CheckDomainPayloads(accountRegisters *registers.AccountRegisters) error {
	domainPayloads := getDomainPayloads(accountRegisters)

	if len(domainPayloads) == 0 {
		return nil
	}

	var errs []error

	slabIndexLength := len(atree.SlabIndex{})

	for id, v := range domainPayloads {
		if _, exist := allStorageMapDomainsSet[id.Key]; !exist {
			errs = append(errs, fmt.Errorf("found unexpected domain: %s", id.Key))
			continue
		}
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

func getSlabIDsFromRegisters(registers registers.Registers) ([]atree.SlabID, error) {
	slabIDs := make([]atree.SlabID, 0, registers.Count())

	err := registers.ForEach(func(owner string, key string, value []byte) error {
		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		slabID := atree.NewSlabID(
			atree.Address([]byte(owner)),
			atree.SlabIndex([]byte(key[1:])),
		)

		slabIDs = append(slabIDs, slabID)

		return nil
	})
	if err != nil {
		return nil, err
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

func loadAtreeSlabsInRegisterStorage(
	storage *runtime.Storage,
	registers registers.Registers,
	nWorkers int,
) error {
	slabIDs, err := getSlabIDsFromRegisters(registers)
	if err != nil {
		return err
	}

	return storage.PersistentSlabStorage.BatchPreload(slabIDs, nWorkers)
}

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	registers registers.Registers,
	nWorkers int,
	needToPreloadAtreeSlabs bool,
) error {

	if needToPreloadAtreeSlabs {
		err := loadAtreeSlabsInRegisterStorage(storage, registers, nWorkers)
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
	stdlib.PathCapabilityStorageDomain,
	stdlib.AccountCapabilityStorageDomain,
}

var allStorageMapDomainsSet = map[string]struct{}{}

func init() {
	for _, domain := range allStorageMapDomains {
		allStorageMapDomainsSet[domain] = struct{}{}
	}
}
