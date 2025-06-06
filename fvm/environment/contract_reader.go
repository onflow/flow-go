package environment

import (
	"fmt"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ContractReader provide read access to contracts.
type ContractReader struct {
	tracer                tracing.TracerSpan
	meter                 Meter
	accounts              Accounts
	cryptoContractAddress common.Address
}

func NewContractReader(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
	cryptoContractAddress common.Address,
) *ContractReader {
	return &ContractReader{
		tracer:                tracer,
		meter:                 meter,
		accounts:              accounts,
		cryptoContractAddress: cryptoContractAddress,
	}
}

func (reader *ContractReader) GetAccountContractNames(
	runtimeAddress common.Address,
) (
	[]string,
	error,
) {
	defer reader.tracer.StartChildSpan(
		trace.FVMEnvGetAccountContractNames).End()

	err := reader.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetAccountContractNames,
			Intensity: 1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get account contract names failed: %w", err)
	}

	address := flow.ConvertAddress(runtimeAddress)

	return reader.accounts.GetContractNames(address)
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

	err := reader.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindResolveLocation,
			Intensity: 1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("resolve location failed: %w", err)
	}

	return ResolveLocation(
		identifiers,
		location,
		reader.accounts.GetContractNames,
		reader.cryptoContractAddress,
	)
}

func ResolveLocation(
	identifiers []ast.Identifier,
	location common.Location,
	getContractNames func(flow.Address) ([]string, error),
	cryptoContractAddress common.Address,
) ([]runtime.ResolvedLocation, error) {

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location
	// then return a single resolved location which declares all identifiers.
	if !isAddress {

		// if the location is the Crypto contract,
		// translate it to the address of the Crypto contract on the chain

		if location == stdlib.CryptoContractLocation {
			location = common.AddressLocation{
				Address: cryptoContractAddress,
				Name:    string(stdlib.CryptoContractLocation),
			}
		}

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
		if getContractNames == nil {
			return nil, fmt.Errorf("no identifiers provided")
		}

		address := flow.ConvertAddress(addressLocation.Address)

		contractNames, err := getContractNames(address)
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

func (reader *ContractReader) getCode(
	location common.AddressLocation,
) (
	[]byte,
	error,
) {
	defer reader.tracer.StartChildSpan(trace.FVMEnvGetCode).End()

	err := reader.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetCode,
			Intensity: 1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	add, err := reader.accounts.GetContract(location.Name, flow.ConvertAddress(location.Address))
	if err != nil {
		return nil, fmt.Errorf("get code failed: %w", err)
	}

	return add, nil
}

func (reader *ContractReader) GetCode(
	location runtime.Location,
) (
	[]byte,
	error,
) {
	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, errors.NewInvalidLocationErrorf(
			location,
			"expecting an AddressLocation, but other location types are passed")
	}

	return reader.getCode(contractLocation)
}

func (reader *ContractReader) GetAccountContractCode(
	location common.AddressLocation,
) (
	[]byte,
	error,
) {
	defer reader.tracer.StartChildSpan(
		trace.FVMEnvGetAccountContractCode).End()

	err := reader.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetAccountContractCode,
			Intensity: 1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	code, err := reader.getCode(location)
	if err != nil {
		return nil, fmt.Errorf("get account contract code failed: %w", err)
	}

	return code, nil
}
