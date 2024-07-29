package stdlib

import (
	"fmt"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"go.opentelemetry.io/otel/attribute"
)

// checkingInterface is a runtime.Interface implementation
// that can be used for ParseAndCheckProgram.
// It is not suitable for execution.
type checkingInterface struct {
	SystemContractCodes map[common.AddressLocation][]byte
	Programs            map[runtime.Location]*interpreter.Program
}

var _ runtime.Interface = &checkingInterface{}

func (*checkingInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) (
	[]runtime.ResolvedLocation,
	error,
) {

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

	if len(identifiers) == 0 {
		return nil, fmt.Errorf("no identifiers provided")
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

func (r *checkingInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	program *interpreter.Program,
	err error,
) {
	if r.Programs == nil {
		r.Programs = map[runtime.Location]*interpreter.Program{}
	}

	var ok bool
	program, ok = r.Programs[location]
	if ok {
		return
	}

	program, err = load()

	// NOTE: important: still set empty program,
	// even if error occurred

	r.Programs[location] = program

	return
}

func (r *checkingInterface) GetAccountContractCode(location common.AddressLocation) (code []byte, err error) {
	return r.SystemContractCodes[location], nil
}

func (*checkingInterface) MeterMemory(_ common.MemoryUsage) error {
	// NO-OP
	return nil
}

func (*checkingInterface) RecoverProgram(_ *ast.Program, _ runtime.Location) (*ast.Program, error) {
	// NO-OP
	return nil, nil
}

func (*checkingInterface) MeterComputation(_ common.ComputationKind, _ uint) error {
	panic("unexpected call to MeterComputation")
}

func (*checkingInterface) ComputationUsed() (uint64, error) {
	panic("unexpected call to ComputationUsed")
}

func (*checkingInterface) MemoryUsed() (uint64, error) {
	panic("unexpected call to MemoryUsed")
}

func (*checkingInterface) InteractionUsed() (uint64, error) {
	panic("unexpected call to InteractionUsed")
}

func (*checkingInterface) GetCode(_ runtime.Location) ([]byte, error) {
	panic("unexpected call to GetCode")
}

func (*checkingInterface) SetInterpreterSharedState(_ *interpreter.SharedState) {
	panic("unexpected call to SetInterpreterSharedState")
}

func (*checkingInterface) GetInterpreterSharedState() *interpreter.SharedState {
	panic("unexpected call to GetInterpreterSharedState")
}

func (*checkingInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected call to GetValue")
}

func (*checkingInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected call to SetValue")
}

func (*checkingInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected call to ValueExists")
}

func (*checkingInterface) AllocateSlabIndex(_ []byte) (atree.SlabIndex, error) {
	panic("unexpected call to AllocateSlabIndex")
}

func (*checkingInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected call to CreateAccount")
}

func (*checkingInterface) AddAccountKey(
	_ runtime.Address,
	_ *runtime.PublicKey,
	_ runtime.HashAlgorithm,
	_ int,
) (*runtime.AccountKey, error) {
	panic("unexpected call to AddAccountKey")
}

func (*checkingInterface) GetAccountKey(_ runtime.Address, _ uint32) (*runtime.AccountKey, error) {
	panic("unexpected call to GetAccountKey")
}

func (*checkingInterface) AccountKeysCount(_ runtime.Address) (uint32, error) {
	panic("unexpected call to AccountKeysCount")
}

func (*checkingInterface) RevokeAccountKey(_ runtime.Address, _ uint32) (*runtime.AccountKey, error) {
	panic("unexpected call to RevokeAccountKey")
}

func (*checkingInterface) UpdateAccountContractCode(_ common.AddressLocation, _ []byte) (err error) {
	panic("unexpected call to UpdateAccountContractCode")
}

func (*checkingInterface) RemoveAccountContractCode(_ common.AddressLocation) (err error) {
	panic("unexpected call to RemoveAccountContractCode")
}

func (*checkingInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected call to GetSigningAccounts")
}

func (*checkingInterface) ProgramLog(_ string) error {
	panic("unexpected call to ProgramLog")
}

func (*checkingInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected call to EmitEvent")
}

func (*checkingInterface) GenerateUUID() (uint64, error) {
	panic("unexpected call to GenerateUUID")
}

func (*checkingInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected call to DecodeArgument")
}

func (*checkingInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected call to GetCurrentBlockHeight")
}

func (*checkingInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected call to GetBlockAtHeight")
}

func (*checkingInterface) ReadRandom(_ []byte) error {
	panic("unexpected call to ReadRandom")
}

func (*checkingInterface) VerifySignature(
	_ []byte,
	_ string,
	_ []byte,
	_ []byte,
	_ runtime.SignatureAlgorithm,
	_ runtime.HashAlgorithm,
) (bool, error) {
	panic("unexpected call to VerifySignature")
}

func (*checkingInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected call to Hash")
}

func (*checkingInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected call to GetAccountBalance")
}

func (*checkingInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected call to GetAccountAvailableBalance")
}

func (*checkingInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected call to GetStorageUsed")
}

func (*checkingInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected call to GetStorageCapacity")
}

func (*checkingInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected call to ImplementationDebugLog")
}

func (*checkingInterface) ValidatePublicKey(_ *runtime.PublicKey) error {
	panic("unexpected call to ValidatePublicKey")
}

func (*checkingInterface) GetAccountContractNames(_ runtime.Address) ([]string, error) {
	panic("unexpected call to GetAccountContractNames")
}

func (*checkingInterface) RecordTrace(_ string, _ runtime.Location, _ time.Duration, _ []attribute.KeyValue) {
	panic("unexpected call to RecordTrace")
}

func (*checkingInterface) BLSVerifyPOP(_ *runtime.PublicKey, _ []byte) (bool, error) {
	panic("unexpected call to BLSVerifyPOP")
}

func (*checkingInterface) BLSAggregateSignatures(_ [][]byte) ([]byte, error) {
	panic("unexpected call to BLSAggregateSignatures")
}

func (*checkingInterface) BLSAggregatePublicKeys(_ []*runtime.PublicKey) (*runtime.PublicKey, error) {
	panic("unexpected call to BLSAggregatePublicKeys")
}

func (*checkingInterface) ResourceOwnerChanged(
	_ *interpreter.Interpreter,
	_ *interpreter.CompositeValue,
	_ common.Address,
	_ common.Address,
) {
	panic("unexpected call to ResourceOwnerChanged")
}

func (*checkingInterface) GenerateAccountID(_ common.Address) (uint64, error) {
	panic("unexpected call to GenerateAccountID")
}
