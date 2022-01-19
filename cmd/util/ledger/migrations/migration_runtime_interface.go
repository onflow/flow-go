package migrations

import (
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/opentracing/opentracing-go"
)

type migrationRuntimeInterface struct {
	accounts state.Accounts
	programs *programs.Programs
}

func (m migrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	panic("unexpected ResolveLocation call")
}

func (m migrationRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	panic("unexpected GetCode call")
}

func (m migrationRuntimeInterface) GetProgram(location runtime.Location) (*interpreter.Program, error) {
	panic("unexpected GetProgram call")
}

func (m migrationRuntimeInterface) SetProgram(location runtime.Location, program *interpreter.Program) error {
	panic("unexpected SetProgram call")
}

func (m migrationRuntimeInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected GetValue call")
}

func (m migrationRuntimeInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected SetValue call")
}

func (m migrationRuntimeInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected CreateAccount call")
}

func (m migrationRuntimeInterface) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	panic("unexpected AddEncodedAccountKey call")
}

func (m migrationRuntimeInterface) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	panic("unexpected RevokeEncodedAccountKey call")
}

func (m migrationRuntimeInterface) AddAccountKey(
	_ runtime.Address,
	_ *runtime.PublicKey,
	_ runtime.HashAlgorithm,
	_ int,
) (*runtime.AccountKey, error) {
	panic("unexpected AddAccountKey call")
}

func (m migrationRuntimeInterface) GetAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected GetAccountKey call")
}

func (m migrationRuntimeInterface) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected RevokeAccountKey call")
}

func (m migrationRuntimeInterface) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	panic("unexpected UpdateAccountContractCode call")
}

func (m migrationRuntimeInterface) GetAccountContractCode(
	address runtime.Address,
	name string,
) (code []byte, err error) {
	panic("unexpected GetAccountContractCode call")
}

func (m migrationRuntimeInterface) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	panic("unexpected RemoveAccountContractCode call")
}

func (m migrationRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected GetSigningAccounts call")
}

func (m migrationRuntimeInterface) ProgramLog(_ string) error {
	panic("unexpected ProgramLog call")
}

func (m migrationRuntimeInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected EmitEvent call")
}

func (m migrationRuntimeInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected ValueExists call")
}

func (m migrationRuntimeInterface) GenerateUUID() (uint64, error) {
	panic("unexpected GenerateUUID call")
}

func (m migrationRuntimeInterface) GetComputationLimit() uint64 {
	panic("unexpected GetComputationLimit call")
}

func (m migrationRuntimeInterface) SetComputationUsed(_ uint64) error {
	panic("unexpected SetComputationUsed call")
}

func (m migrationRuntimeInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected DecodeArgument call")
}

func (m migrationRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected GetCurrentBlockHeight call")
}

func (m migrationRuntimeInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected GetBlockAtHeight call")
}

func (m migrationRuntimeInterface) UnsafeRandom() (uint64, error) {
	panic("unexpected UnsafeRandom call")
}

func (m migrationRuntimeInterface) VerifySignature(
	_ []byte,
	_ string,
	_ []byte,
	_ []byte,
	_ runtime.SignatureAlgorithm,
	_ runtime.HashAlgorithm,
) (bool, error) {
	panic("unexpected VerifySignature call")
}

func (m migrationRuntimeInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected Hash call")
}

func (m migrationRuntimeInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountBalance call")
}

func (m migrationRuntimeInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountAvailableBalance call")
}

func (m migrationRuntimeInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageUsed call")
}

func (m migrationRuntimeInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageCapacity call")
}

func (m migrationRuntimeInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected ImplementationDebugLog call")
}

func (m migrationRuntimeInterface) ValidatePublicKey(_ *runtime.PublicKey) (bool, error) {
	panic("unexpected ValidatePublicKey call")
}

func (m migrationRuntimeInterface) GetAccountContractNames(_ runtime.Address) ([]string, error) {
	panic("unexpected GetAccountContractNames call")
}

func (m migrationRuntimeInterface) AllocateStorageIndex(_ []byte) (atree.StorageIndex, error) {
	panic("unexpected AllocateStorageIndex call")
}

func (m migrationRuntimeInterface) AggregateBLSPublicKeys(_ []*runtime.PublicKey) (*runtime.PublicKey, error) {
	panic("unexpected AggregateBLSPublicKeys call")
}

func (m migrationRuntimeInterface) BLSVerifyPOP(pk *runtime.PublicKey, s []byte) (bool, error) {
	panic("unexpected BLSVerifyPOP call")
}

func (m migrationRuntimeInterface) AggregateBLSSignatures(sigs [][]byte) ([]byte, error) {
	panic("unexpected AggregateBLSSignatures call")
}

func (m migrationRuntimeInterface) ResourceOwnerChanged(resource *interpreter.CompositeValue, oldOwner common.Address, newOwner common.Address) {
	panic("unexpected ResourceOwnerChanged call")
}

func (m migrationRuntimeInterface) RecordTrace(operation string, location common.Location, duration time.Duration, logs []opentracing.LogRecord) {
	panic("unexpected RecordTrace call")
}
