package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ContractReader provide read access to contracts.
type ContractReader struct {
	tracer tracing.TracerSpan
	meter  Meter

	accounts Accounts
}

func NewContractReader(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
) *ContractReader {
	return &ContractReader{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
	}
}

func (reader *ContractReader) GetAccountContractNames(
	address common.Address,
) (
	[]string,
	error,
) {
	defer reader.tracer.StartChildSpan(
		trace.FVMEnvGetAccountContractNames).End()

	err := reader.meter.MeterComputation(
		ComputationKindGetAccountContractNames,
		1)
	if err != nil {
		return nil, fmt.Errorf("get account contract names failed: %w", err)
	}

	a := flow.Address(address)

	freezeError := reader.accounts.CheckAccountNotFrozen(a)
	if freezeError != nil {
		return nil, fmt.Errorf(
			"get account contract names failed: %w",
			freezeError)
	}

	return reader.accounts.GetContractNames(a)
}

func (reader *ContractReader) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) (
	[]runtime.ResolvedLocation,
	error,
) {
	defer reader.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvResolveLocation).End()

	err := reader.meter.MeterComputation(ComputationKindResolveLocation, 1)
	if err != nil {
		return nil, fmt.Errorf("resolve location failed: %w", err)
	}

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location
	// (`import Crypto`), then return a single resolved location which declares
	// all identifiers.
	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address
	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)

		err := reader.accounts.CheckAccountNotFrozen(address)
		if err != nil {
			return nil, fmt.Errorf(
				"resolving location's account frozen check failed: %w",
				err)
		}

		contractNames, err := reader.accounts.GetContractNames(address)
		if err != nil {
			return nil, fmt.Errorf("resolving location failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]ast.Identifier, len(contractNames))

		for i := range identifiers {
			identifiers[i] = runtime.Identifier{
				Identifier: contractNames[i],
			}
		}
	}

	// return one resolved location per identifier.
	// each resolved location is an address contract location
	resolvedLocations := make([]runtime.ResolvedLocation, len(identifiers))
	for i := range resolvedLocations {
		identifier := identifiers[i]
		resolvedLocations[i] = runtime.ResolvedLocation{
			Location: common.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations, nil
}

func (reader *ContractReader) GetCode(
	location runtime.Location,
) (
	[]byte,
	error,
) {
	defer reader.tracer.StartChildSpan(trace.FVMEnvGetCode).End()

	err := reader.meter.MeterComputation(ComputationKindGetCode, 1)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, errors.NewInvalidLocationErrorf(
			location,
			"expecting an AddressLocation, but other location types are passed")
	}

	address := flow.Address(contractLocation.Address)

	err = reader.accounts.CheckAccountNotFrozen(address)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	add, err := reader.accounts.GetContract(contractLocation.Name, address)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	return add, nil
}

func (reader *ContractReader) GetAccountContractCode(
	address common.Address,
	name string,
) (
	code []byte,
	err error,
) {
	defer reader.tracer.StartChildSpan(
		trace.FVMEnvGetAccountContractCode).End()

	err = reader.meter.MeterComputation(
		ComputationKindGetAccountContractCode,
		1)
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	code, err = reader.GetCode(common.AddressLocation{
		Address: address,
		Name:    name,
	})
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	return code, nil
}
