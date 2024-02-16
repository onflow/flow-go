package util

import (
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// MigrationRuntimeInterface is a runtime interface that can be used in migrations.
type MigrationRuntimeInterface struct {
	RuntimeInterfaceConfig
	Accounts      environment.Accounts
	Programs      *environment.Programs
	ProgramErrors map[common.Location]error
}

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

		contractNames, err := m.Accounts.GetContractNames(address)
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
		return nil, fmt.Errorf("GetCode failed: expected AddressLocation")
	}

	add, err := m.Accounts.GetContract(contractLocation.Name, flow.Address(contractLocation.Address))
	if err != nil {
		return nil, fmt.Errorf("GetCode failed: %w", err)
	}

	return add, nil
}

func (m *MigrationRuntimeInterface) GetAccountContractCode(
	location common.AddressLocation,
) (code []byte, err error) {
	// First look for staged contracts.
	// If not found, then fall back to getting the code from storage.
	if m.GetContractCodeFunc != nil {
		code, err := m.GetContractCodeFunc(location)
		if err != nil || code != nil {
			// If the code was found, or if an error occurred, then return.
			return code, err
		}
	}

	return m.Accounts.GetContract(location.Name, flow.Address(location.Address))
}

func (m *MigrationRuntimeInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (*interpreter.Program, error) {
	return m.Programs.GetOrLoadProgram(
		location,
		func() (*interpreter.Program, error) {
			// If the program is already known to be invalid,
			// then return the error immediately,
			// without attempting to load the program again
			if err, ok := m.ProgramErrors[location]; ok {
				return nil, err
			}

			// Otherwise, load the program.
			// If an error occurs, then record it for subsequent calls
			program, err := load()
			if err != nil {
				m.ProgramErrors[location] = err
			}

			return program, err
		},
	)
}

func (m *MigrationRuntimeInterface) MeterMemory(_ common.MemoryUsage) error {
	return nil
}

func (m *MigrationRuntimeInterface) MeterComputation(_ common.ComputationKind, _ uint) error {
	return nil
}

func (m *MigrationRuntimeInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected GetValue call")
}

func (m *MigrationRuntimeInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected SetValue call")
}

func (m *MigrationRuntimeInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected CreateAccount call")
}

func (m *MigrationRuntimeInterface) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	panic("unexpected AddEncodedAccountKey call")
}

func (m *MigrationRuntimeInterface) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	panic("unexpected RevokeEncodedAccountKey call")
}

func (m *MigrationRuntimeInterface) AddAccountKey(
	_ runtime.Address,
	_ *runtime.PublicKey,
	_ runtime.HashAlgorithm,
	_ int,
) (*runtime.AccountKey, error) {
	panic("unexpected AddAccountKey call")
}

func (m *MigrationRuntimeInterface) GetAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected GetAccountKey call")
}

func (m *MigrationRuntimeInterface) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected RevokeAccountKey call")
}

func (m *MigrationRuntimeInterface) UpdateAccountContractCode(_ common.AddressLocation, _ []byte) (err error) {
	panic("unexpected UpdateAccountContractCode call")
}

func (m *MigrationRuntimeInterface) RemoveAccountContractCode(common.AddressLocation) (err error) {
	panic("unexpected RemoveAccountContractCode call")
}

func (m *MigrationRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected GetSigningAccounts call")
}

func (m *MigrationRuntimeInterface) ProgramLog(_ string) error {
	panic("unexpected ProgramLog call")
}

func (m *MigrationRuntimeInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected EmitEvent call")
}

func (m *MigrationRuntimeInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected ValueExists call")
}

func (m *MigrationRuntimeInterface) GenerateUUID() (uint64, error) {
	panic("unexpected GenerateUUID call")
}

func (m *MigrationRuntimeInterface) GetComputationLimit() uint64 {
	panic("unexpected GetComputationLimit call")
}

func (m *MigrationRuntimeInterface) SetComputationUsed(_ uint64) error {
	panic("unexpected SetComputationUsed call")
}

func (m *MigrationRuntimeInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected DecodeArgument call")
}

func (m *MigrationRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected GetCurrentBlockHeight call")
}

func (m *MigrationRuntimeInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected GetBlockAtHeight call")
}

func (m *MigrationRuntimeInterface) ReadRandom([]byte) error {
	panic("unexpected ReadRandom call")
}

func (m *MigrationRuntimeInterface) VerifySignature(
	_ []byte,
	_ string,
	_ []byte,
	_ []byte,
	_ runtime.SignatureAlgorithm,
	_ runtime.HashAlgorithm,
) (bool, error) {
	panic("unexpected VerifySignature call")
}

func (m *MigrationRuntimeInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected Hash call")
}

func (m *MigrationRuntimeInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountBalance call")
}

func (m *MigrationRuntimeInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountAvailableBalance call")
}

func (m *MigrationRuntimeInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageUsed call")
}

func (m *MigrationRuntimeInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageCapacity call")
}

func (m *MigrationRuntimeInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected ImplementationDebugLog call")
}

func (m *MigrationRuntimeInterface) ValidatePublicKey(_ *runtime.PublicKey) error {
	panic("unexpected ValidatePublicKey call")
}

func (m *MigrationRuntimeInterface) GetAccountContractNames(_ runtime.Address) ([]string, error) {
	panic("unexpected GetAccountContractNames call")
}

func (m *MigrationRuntimeInterface) AllocateStorageIndex(_ []byte) (atree.StorageIndex, error) {
	panic("unexpected AllocateStorageIndex call")
}

func (m *MigrationRuntimeInterface) ComputationUsed() (uint64, error) {
	panic("unexpected ComputationUsed call")
}

func (m *MigrationRuntimeInterface) MemoryUsed() (uint64, error) {
	panic("unexpected MemoryUsed call")
}

func (m *MigrationRuntimeInterface) InteractionUsed() (uint64, error) {
	panic("unexpected InteractionUsed call")
}

func (m *MigrationRuntimeInterface) SetInterpreterSharedState(_ *interpreter.SharedState) {
	panic("unexpected SetInterpreterSharedState call")
}

func (m *MigrationRuntimeInterface) GetInterpreterSharedState() *interpreter.SharedState {
	panic("unexpected GetInterpreterSharedState call")
}

func (m *MigrationRuntimeInterface) AccountKeysCount(_ runtime.Address) (uint64, error) {
	panic("unexpected AccountKeysCount call")
}

func (m *MigrationRuntimeInterface) BLSVerifyPOP(_ *runtime.PublicKey, _ []byte) (bool, error) {
	panic("unexpected BLSVerifyPOP call")
}

func (m *MigrationRuntimeInterface) BLSAggregateSignatures(_ [][]byte) ([]byte, error) {
	panic("unexpected BLSAggregateSignatures call")
}

func (m *MigrationRuntimeInterface) BLSAggregatePublicKeys(_ []*runtime.PublicKey) (*runtime.PublicKey, error) {
	panic("unexpected BLSAggregatePublicKeys call")
}

func (m *MigrationRuntimeInterface) ResourceOwnerChanged(
	_ *interpreter.Interpreter,
	_ *interpreter.CompositeValue,
	_ common.Address,
	_ common.Address,
) {
	panic("unexpected ResourceOwnerChanged call")
}

func (m *MigrationRuntimeInterface) GenerateAccountID(_ common.Address) (uint64, error) {
	panic("unexpected GenerateAccountID call")
}

func (m *MigrationRuntimeInterface) RecordTrace(_ string, _ runtime.Location, _ time.Duration, _ []attribute.KeyValue) {
	panic("unexpected RecordTrace call")
}

type RuntimeInterfaceConfig struct {
	// GetContractCodeFunc allows for injecting extra logic for code lookup
	GetContractCodeFunc func(location runtime.Location) ([]byte, error)
}
