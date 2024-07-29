package util

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

type GetContractCodeFunc func(location common.AddressLocation) ([]byte, error)

type GetContractNamesFunc func(address flow.Address) ([]string, error)

type GetOrLoadProgramFunc func(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	*interpreter.Program,
	error,
)

type GerOrLoadProgramListenerFunc func(
	location runtime.Location,
	program *interpreter.Program,
	err error,
)

// MigrationRuntimeInterface is a runtime interface that can be used in migrations.
// It only allows parsing and checking of contracts.
type MigrationRuntimeInterface struct {
	runtime.EmptyRuntimeInterface
	GetContractCodeFunc          GetContractCodeFunc
	GetContractNamesFunc         GetContractNamesFunc
	GetOrLoadProgramFunc         GetOrLoadProgramFunc
	GetOrLoadProgramListenerFunc GerOrLoadProgramListenerFunc
}

var _ runtime.Interface = &MigrationRuntimeInterface{}

func NewMigrationRuntimeInterface(
	getCodeFunc GetContractCodeFunc,
	getContractNamesFunc GetContractNamesFunc,
	getOrLoadProgramFunc GetOrLoadProgramFunc,
	getOrLoadProgramListenerFunc GerOrLoadProgramListenerFunc,
) *MigrationRuntimeInterface {
	return &MigrationRuntimeInterface{
		GetContractCodeFunc:          getCodeFunc,
		GetContractNamesFunc:         getContractNamesFunc,
		GetOrLoadProgramFunc:         getOrLoadProgramFunc,
		GetOrLoadProgramListenerFunc: getOrLoadProgramListenerFunc,
	}
}

var _ runtime.Interface = &MigrationRuntimeInterface{}

func (m *MigrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.
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

		getContractNames := m.GetContractNamesFunc
		if getContractNames == nil {
			return nil, errors.New("GetContractNamesFunc missing")
		}

		contractNames, err := getContractNames(address)
		if err != nil {
			return nil, fmt.Errorf("ResolveLocation failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]runtime.Identifier, len(contractNames))

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

func (m *MigrationRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("GetCode failed: expected AddressLocation, got %T", location)
	}

	return m.GetAccountContractCode(contractLocation)
}

func (m *MigrationRuntimeInterface) GetAccountContractCode(
	location common.AddressLocation,
) (code []byte, err error) {
	getContractCode := m.GetContractCodeFunc
	if getContractCode == nil {
		return nil, fmt.Errorf("GetCodeFunc missing")
	}

	return getContractCode(location)
}

func (m *MigrationRuntimeInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	program *interpreter.Program,
	err error,
) {
	getOrLoadProgram := m.GetOrLoadProgramFunc
	if getOrLoadProgram == nil {
		return nil, errors.New("GetOrLoadProgramFunc missing")
	}

	listener := m.GetOrLoadProgramListenerFunc
	if listener != nil {
		defer func() {
			listener(location, program, err)
		}()
	}

	return getOrLoadProgram(location, load)
}

type migrationTransactionPreparer struct {
	state.NestedTransactionPreparer
	derived.DerivedTransactionPreparer
}
