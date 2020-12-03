package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type StorageCapacityResolver func(state.Ledger, flow.Address) (uint64, error)

func CreateResourceStorageCapacityResolver() StorageCapacityResolver {
	return func(l state.Ledger, address flow.Address) (uint64, error) {
		key := fmt.Sprintf("%s\x1F%s", "storage", "storageCapacity") // StorageCapacity resource key. Its the /storage/storageCapacity path
		resource, err := l.Get(string(address.Bytes()), "", key)
		if err != nil {
			return 0, fmt.Errorf("could not load storage capacity resource: %w", err)
		}

		storedData, version := interpreter.StripMagic(resource)
		commonAddress := common.BytesToAddress(address.Bytes())
		storedValue, err := interpreter.DecodeValue(storedData, &commonAddress, []string{key}, version)
		if err != nil {
			return 0, nil
		}

		composite, ok := storedValue.(*interpreter.CompositeValue)
		if !ok {
			return 0, nil
		}

		// where do I get the storageFeesAddress?
		//expectedTypeID := fmt.Sprintf("A.%s.StorageFees.StorageCapacity", storageFeesAddress.Hex())
		//if string(composite.TypeID) != expectedTypeID {
		//	return 0, nil
		//}

		holderAddressValue, ok := composite.Fields["address"]
		if !ok {
			return 0, nil
		}
		holderAddress, ok := holderAddressValue.(interpreter.AddressValue)
		if !ok {
			return 0, nil
		}
		if holderAddress.ToAddress().Hex() != commonAddress.Hex() {
			return 0, nil
		}

		capacityValue, ok := composite.Fields["storageCapacity"]
		if !ok {
			return 0, nil
		}
		capacity, ok := capacityValue.(interpreter.UInt64Value)
		if !ok {
			return 0, nil
		}

		return uint64(capacity), nil
	}
}
