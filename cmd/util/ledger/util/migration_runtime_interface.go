package util

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
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
	chainID                      flow.ChainID
	CryptoContractAddress        common.Address
	GetContractCodeFunc          GetContractCodeFunc
	GetContractNamesFunc         GetContractNamesFunc
	GetOrLoadProgramFunc         GetOrLoadProgramFunc
	GetOrLoadProgramListenerFunc GerOrLoadProgramListenerFunc
}

var _ runtime.Interface = &MigrationRuntimeInterface{}

func NewMigrationRuntimeInterface(
	chainID flow.ChainID,
	cryptoContractAddress common.Address,
	getCodeFunc GetContractCodeFunc,
	getContractNamesFunc GetContractNamesFunc,
	getOrLoadProgramFunc GetOrLoadProgramFunc,
	getOrLoadProgramListenerFunc GerOrLoadProgramListenerFunc,
) *MigrationRuntimeInterface {
	return &MigrationRuntimeInterface{
		chainID:                      chainID,
		CryptoContractAddress:        cryptoContractAddress,
		GetContractCodeFunc:          getCodeFunc,
		GetContractNamesFunc:         getContractNamesFunc,
		GetOrLoadProgramFunc:         getOrLoadProgramFunc,
		GetOrLoadProgramListenerFunc: getOrLoadProgramListenerFunc,
	}
}

func (m *MigrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {

	return environment.ResolveLocation(
		identifiers,
		location,
		m.GetContractNamesFunc,
		m.CryptoContractAddress,
	)
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

func (m *MigrationRuntimeInterface) RecoverProgram(
	program *ast.Program,
	location common.Location,
) ([]byte, error) {
	return environment.RecoverProgram(m.chainID, program, location)
}

type migrationTransactionPreparer struct {
	state.NestedTransactionPreparer
	derived.DerivedTransactionPreparer
}
