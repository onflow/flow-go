package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type StorageCapacityResolver func(state.Ledger, flow.Address, Context) (uint64, error)

func ResourceStorageCapacityResolver(l state.Ledger, address flow.Address, ctx Context) (uint64, error) {
	storageReservation, err := getAccountsStorageReservation(l, address, ctx)
	if err != nil {
		return 0, err
	}

	minimumStorageReservation, bytesPerFLOW, err := getStorageFeeParameters(l, ctx)
	if err != nil {
		return 0, err
	}
	if storageReservation < minimumStorageReservation {
		return uint64(0), nil
	}

	storageCapacityValue := storageReservation.Mul(bytesPerFLOW).(interpreter.UFix64Value).ToInt()
	return uint64(storageCapacityValue), nil
}

func getStorageFeeParameters(l state.Ledger, ctx Context) (interpreter.UFix64Value, interpreter.UFix64Value, error) {
	flowStorageFeesId := flow.RegisterID{
		Owner:      string(ctx.Chain.ServiceAddress().Bytes()),
		Controller: "",
		Key:        fmt.Sprintf("contract\x1F%s", "FlowStorageFees"),
	}

	composite, err := getCompositeFromRegister(l, flowStorageFeesId)
	if err != nil {
		return 0.0, 0.0, err
	}

	minimumStorageReservationField, ok := composite.Fields["minimumStorageReservation"]
	if !ok {
		return 0.0, 0.0, fmt.Errorf("expected field not present on the FlowStorageFees contract")
	}
	minimumStorageReservation, ok := minimumStorageReservationField.(interpreter.UFix64Value)
	if !ok {
		return 0.0, 0.0, fmt.Errorf("field on the FlowStorageFees contract not in expected format")
	}

	bytesPerFlowField, ok := composite.Fields["storageBytesPerReservedFLOW"]
	if !ok {
		return 0.0, 0.0, fmt.Errorf("expected field not present on the FlowStorageFees contract")
	}
	bytesPerFlow, ok := bytesPerFlowField.(interpreter.UFix64Value)
	if !ok {
		return 0.0, 0.0, fmt.Errorf("field on the FlowStorageFees contract not in expected format")
	}

	return minimumStorageReservation, bytesPerFlow, nil
}

func getAccountsStorageReservation(l state.Ledger, address flow.Address, ctx Context) (interpreter.UFix64Value, error) {
	storageReservationRegisterId := flow.RegisterID{
		Owner:      string(address.Bytes()),
		Controller: "",
		// StorageReservation resource key. Its the /storage/storageReservation path
		Key: fmt.Sprintf("%s\x1F%s", "storage", "flowStorageReservation"),
	}

	composite, err := getCompositeFromRegister(l, storageReservationRegisterId)
	if err != nil {
		return 0, nil
	}

	expectedTypeID := fmt.Sprintf("A.%s.FlowStorageFees.StorageReservation", ctx.Chain.ServiceAddress().Hex())
	if string(composite.TypeID) != expectedTypeID {
		return 0, nil
	}

	holderAddressValue, ok := composite.Fields["ownerAddress"]
	if !ok {
		return 0, nil
	}
	holderAddress, ok := holderAddressValue.(interpreter.AddressValue)
	if !ok {
		return 0, nil
	}
	if holderAddress.ToAddress().Hex() != address.Hex() {
		return 0, nil
	}

	reservedTokens, ok := composite.Fields["reservedTokens"]
	if !ok {
		return 0, nil
	}
	reservedTokensValue, ok := reservedTokens.(*interpreter.CompositeValue)
	if !ok {
		return 0, nil
	}
	balanceField, ok := reservedTokensValue.Fields["balance"]
	if !ok {
		return 0, nil
	}
	balance, ok := balanceField.(interpreter.UFix64Value)
	if !ok {
		return 0, nil
	}
	return balance, nil
}

func getCompositeFromRegister(l state.Ledger, id flow.RegisterID) (*interpreter.CompositeValue, error) {
	resource, err := l.Get(id.Owner, id.Controller, id.Key)
	if err != nil {
		return nil, fmt.Errorf("could not load storage capacity resource at %s: %w", id.String(), err)
	}
	storedData, version := interpreter.StripMagic(resource)
	commonAddress := common.BytesToAddress([]byte(id.Owner))
	storedValue, err := interpreter.DecodeValue(storedData, &commonAddress, []string{id.Key}, version)
	if err != nil {
		return nil, err
	}

	composite, ok := storedValue.(*interpreter.CompositeValue)
	if !ok {
		return nil, fmt.Errorf("value at %s is not a composite", id.String())
	}
	return composite, nil
}
