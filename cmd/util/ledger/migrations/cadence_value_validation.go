package migrations

import (
	"fmt"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
)

var nopMemoryGauge = util.NopMemoryGauge{}

// TODO: optimize memory by reusing payloads snapshot created for migration
func validateCadenceValues(
	address common.Address,
	oldPayloads []*ledger.Payload,
	newPayloads []*ledger.Payload,
) error {
	// Create all the runtime components we need for comparing Cadence values.
	oldRuntime, err := newReadonlyStorageRuntime(oldPayloads)
	if err != nil {
		return fmt.Errorf("failed to create validator runtime with old payloads: %w", err)
	}

	newRuntime, err := newReadonlyStorageRuntime(newPayloads)
	if err != nil {
		return fmt.Errorf("failed to create validator runtime with new payloads: %w", err)
	}

	// Iterate through all domains and compare cadence values.
	for _, domain := range domains {
		err := validateStorageDomain(address, oldRuntime, newRuntime, domain)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateStorageDomain(
	address common.Address,
	oldRuntime *readonlyStorageRuntime,
	newRuntime *readonlyStorageRuntime,
	domain string,
) error {

	oldStorageMap := oldRuntime.Storage.GetStorageMap(address, domain, false)

	newStorageMap := newRuntime.Storage.GetStorageMap(address, domain, false)

	if oldStorageMap == nil && newStorageMap == nil {
		// No storage for this domain.
		return nil
	}

	if oldStorageMap == nil && newStorageMap != nil {
		return fmt.Errorf("old storage map is nil, new storage map isn't nil")
	}

	if oldStorageMap != nil && newStorageMap == nil {
		return fmt.Errorf("old storage map isn't nil, new storage map is nil")
	}

	if oldStorageMap.Count() != newStorageMap.Count() {
		return fmt.Errorf("old storage map count %d, new storage map count %d", oldStorageMap.Count(), newStorageMap.Count())
	}

	oldIterator := oldStorageMap.Iterator(nopMemoryGauge)
	for {
		key, oldValue := oldIterator.Next()
		if key == nil {
			break
		}

		stringKey, ok := key.(interpreter.StringAtreeValue)
		if !ok {
			return fmt.Errorf("invalid key type %T, expected interpreter.StringAtreeValue", key)
		}

		newValue := newStorageMap.ReadValue(nopMemoryGauge, interpreter.StringStorageMapKey(stringKey))

		if !cadenceValueEqual(oldRuntime.Interpreter, oldValue, newRuntime.Interpreter, newValue) {
			return fmt.Errorf("failed to validate domain %s, key %s: old value %v (%T), new value %v (%T)", domain, key, oldValue, oldValue, newValue, newValue)
		}
	}

	return nil
}

func cadenceValueEqual(
	vInterpreter *interpreter.Interpreter,
	v interpreter.Value,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) bool {
	switch v := v.(type) {
	case *interpreter.ArrayValue:
		return cadenceArrayValueEqual(vInterpreter, v, otherInterpreter, other)

	case *interpreter.CompositeValue:
		return cadenceCompositeValueEqual(vInterpreter, v, otherInterpreter, other)

	case *interpreter.DictionaryValue:
		return cadenceDictionaryValueEqual(vInterpreter, v, otherInterpreter, other)

	case *interpreter.SomeValue:
		return cadenceSomeValueEqual(vInterpreter, v, otherInterpreter, other)

	default:
		oldValue, ok := v.(interpreter.EquatableValue)
		if !ok {
			return false
		}
		return oldValue.Equal(nil, interpreter.EmptyLocationRange, other)
	}
}

func cadenceSomeValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.SomeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) bool {
	otherSome, ok := other.(*interpreter.SomeValue)
	if !ok {
		return false
	}

	innerValue := v.InnerValue(vInterpreter, interpreter.EmptyLocationRange)

	otherInnerValue := otherSome.InnerValue(otherInterpreter, interpreter.EmptyLocationRange)

	return cadenceValueEqual(vInterpreter, innerValue, otherInterpreter, otherInnerValue)
}

func cadenceArrayValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.ArrayValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) bool {
	otherArray, ok := other.(*interpreter.ArrayValue)
	if !ok {
		return false
	}

	count := v.Count()
	if count != otherArray.Count() {
		return false
	}

	if v.Type == nil {
		if otherArray.Type != nil {
			return false
		}
	} else if otherArray.Type == nil ||
		!v.Type.Equal(otherArray.Type) {
		return false
	}

	for i := 0; i < count; i++ {
		element := v.Get(vInterpreter, interpreter.EmptyLocationRange, i)
		otherElement := otherArray.Get(otherInterpreter, interpreter.EmptyLocationRange, i)

		if !cadenceValueEqual(vInterpreter, element, otherInterpreter, otherElement) {
			return false
		}
	}

	return true
}

func cadenceCompositeValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.CompositeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) bool {
	otherComposite, ok := other.(*interpreter.CompositeValue)
	if !ok {
		return false
	}

	if !v.StaticType(vInterpreter).Equal(otherComposite.StaticType(otherInterpreter)) ||
		v.Kind != otherComposite.Kind {
		return false
	}

	var foundMismatch bool
	v.ForEachField(nopMemoryGauge, func(fieldName string, fieldValue interpreter.Value) bool {
		otherFieldValue := otherComposite.GetField(otherInterpreter, interpreter.EmptyLocationRange, fieldName)

		if !cadenceValueEqual(vInterpreter, fieldValue, otherInterpreter, otherFieldValue) {
			foundMismatch = true
			return false
		}

		return true
	})

	return !foundMismatch
}

func cadenceDictionaryValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.DictionaryValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) bool {

	otherDictionary, ok := other.(*interpreter.DictionaryValue)
	if !ok {
		return false
	}

	if v.Count() != otherDictionary.Count() {
		return false
	}

	if !v.Type.Equal(otherDictionary.Type) {
		return false
	}

	oldIterator := v.Iterator()
	for {
		key := oldIterator.NextKey(nopMemoryGauge)
		if key == nil {
			break
		}
		oldValue, oldValueExist := v.Get(vInterpreter, interpreter.EmptyLocationRange, key)
		if !oldValueExist {
			return false
		}
		newValue, newValueExist := otherDictionary.Get(otherInterpreter, interpreter.EmptyLocationRange, key)
		if !newValueExist {
			return false
		}
		if !cadenceValueEqual(vInterpreter, oldValue, otherInterpreter, newValue) {
			return false
		}
	}

	return true
}

type readonlyStorageRuntime struct {
	Interpreter *interpreter.Interpreter
	Storage     *runtime.Storage
}

func newReadonlyStorageRuntime(payloads []*ledger.Payload) (
	*readonlyStorageRuntime,
	error,
) {
	snapshot, err := util.NewPayloadSnapshot(payloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}

	readonlyLedger := util.NewPayloadsReadonlyLedger(snapshot)

	storage := runtime.NewStorage(readonlyLedger, nopMemoryGauge)

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		AccountLinkingEnabled: true,
		// Attachments are enabled everywhere except for Mainnet
		AttachmentsEnabled: true,
		// Capability Controllers are enabled everywhere except for Mainnet
		CapabilityControllersEnabled: true,
	})

	env.Configure(
		&NoopRuntimeInterface{},
		runtime.NewCodesAndPrograms(),
		storage,
		nil,
	)

	inter, err := interpreter.NewInterpreter(nil, nil, env.InterpreterConfig)
	if err != nil {
		return nil, err
	}

	return &readonlyStorageRuntime{
		Interpreter: inter,
		Storage:     storage,
	}, nil
}

// NoopRuntimeInterface is a runtime interface that can be used in migrations.
type NoopRuntimeInterface struct {
}

func (NoopRuntimeInterface) ResolveLocation(_ []runtime.Identifier, _ runtime.Location) ([]runtime.ResolvedLocation, error) {
	panic("unexpected ResolveLocation call")
}

func (NoopRuntimeInterface) GetCode(_ runtime.Location) ([]byte, error) {
	panic("unexpected GetCode call")
}

func (NoopRuntimeInterface) GetAccountContractCode(_ common.AddressLocation) ([]byte, error) {
	panic("unexpected GetAccountContractCode call")
}

func (NoopRuntimeInterface) GetOrLoadProgram(_ runtime.Location, _ func() (*interpreter.Program, error)) (*interpreter.Program, error) {
	panic("unexpected GetOrLoadProgram call")
}

func (NoopRuntimeInterface) MeterMemory(_ common.MemoryUsage) error {
	return nil
}

func (NoopRuntimeInterface) MeterComputation(_ common.ComputationKind, _ uint) error {
	return nil
}

func (NoopRuntimeInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected GetValue call")
}

func (NoopRuntimeInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected SetValue call")
}

func (NoopRuntimeInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected CreateAccount call")
}

func (NoopRuntimeInterface) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	panic("unexpected AddEncodedAccountKey call")
}

func (NoopRuntimeInterface) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	panic("unexpected RevokeEncodedAccountKey call")
}

func (NoopRuntimeInterface) AddAccountKey(_ runtime.Address, _ *runtime.PublicKey, _ runtime.HashAlgorithm, _ int) (*runtime.AccountKey, error) {
	panic("unexpected AddAccountKey call")
}

func (NoopRuntimeInterface) GetAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected GetAccountKey call")
}

func (NoopRuntimeInterface) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected RevokeAccountKey call")
}

func (NoopRuntimeInterface) UpdateAccountContractCode(_ common.AddressLocation, _ []byte) (err error) {
	panic("unexpected UpdateAccountContractCode call")
}

func (NoopRuntimeInterface) RemoveAccountContractCode(common.AddressLocation) (err error) {
	panic("unexpected RemoveAccountContractCode call")
}

func (NoopRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected GetSigningAccounts call")
}

func (NoopRuntimeInterface) ProgramLog(_ string) error {
	panic("unexpected ProgramLog call")
}

func (NoopRuntimeInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected EmitEvent call")
}

func (NoopRuntimeInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected ValueExists call")
}

func (NoopRuntimeInterface) GenerateUUID() (uint64, error) {
	panic("unexpected GenerateUUID call")
}

func (NoopRuntimeInterface) GetComputationLimit() uint64 {
	panic("unexpected GetComputationLimit call")
}

func (NoopRuntimeInterface) SetComputationUsed(_ uint64) error {
	panic("unexpected SetComputationUsed call")
}

func (NoopRuntimeInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected DecodeArgument call")
}

func (NoopRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected GetCurrentBlockHeight call")
}

func (NoopRuntimeInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected GetBlockAtHeight call")
}

func (NoopRuntimeInterface) ReadRandom([]byte) error {
	panic("unexpected ReadRandom call")
}

func (NoopRuntimeInterface) VerifySignature(_ []byte, _ string, _ []byte, _ []byte, _ runtime.SignatureAlgorithm, _ runtime.HashAlgorithm) (bool, error) {
	panic("unexpected VerifySignature call")
}

func (NoopRuntimeInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected Hash call")
}

func (NoopRuntimeInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountBalance call")
}

func (NoopRuntimeInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountAvailableBalance call")
}

func (NoopRuntimeInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageUsed call")
}

func (NoopRuntimeInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageCapacity call")
}

func (NoopRuntimeInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected ImplementationDebugLog call")
}

func (NoopRuntimeInterface) ValidatePublicKey(_ *runtime.PublicKey) error {
	panic("unexpected ValidatePublicKey call")
}

func (NoopRuntimeInterface) GetAccountContractNames(_ runtime.Address) ([]string, error) {
	panic("unexpected GetAccountContractNames call")
}

func (NoopRuntimeInterface) AllocateStorageIndex(_ []byte) (atree.StorageIndex, error) {
	panic("unexpected AllocateStorageIndex call")
}

func (NoopRuntimeInterface) ComputationUsed() (uint64, error) {
	panic("unexpected ComputationUsed call")
}

func (NoopRuntimeInterface) MemoryUsed() (uint64, error) {
	panic("unexpected MemoryUsed call")
}

func (NoopRuntimeInterface) InteractionUsed() (uint64, error) {
	panic("unexpected InteractionUsed call")
}

func (NoopRuntimeInterface) SetInterpreterSharedState(_ *interpreter.SharedState) {
	panic("unexpected SetInterpreterSharedState call")
}

func (NoopRuntimeInterface) GetInterpreterSharedState() *interpreter.SharedState {
	panic("unexpected GetInterpreterSharedState call")
}

func (NoopRuntimeInterface) AccountKeysCount(_ runtime.Address) (uint64, error) {
	panic("unexpected AccountKeysCount call")
}

func (NoopRuntimeInterface) BLSVerifyPOP(_ *runtime.PublicKey, _ []byte) (bool, error) {
	panic("unexpected BLSVerifyPOP call")
}

func (NoopRuntimeInterface) BLSAggregateSignatures(_ [][]byte) ([]byte, error) {
	panic("unexpected BLSAggregateSignatures call")
}

func (NoopRuntimeInterface) BLSAggregatePublicKeys(_ []*runtime.PublicKey) (*runtime.PublicKey, error) {
	panic("unexpected BLSAggregatePublicKeys call")
}

func (NoopRuntimeInterface) ResourceOwnerChanged(_ *interpreter.Interpreter, _ *interpreter.CompositeValue, _ common.Address, _ common.Address) {
	panic("unexpected ResourceOwnerChanged call")
}

func (NoopRuntimeInterface) GenerateAccountID(_ common.Address) (uint64, error) {
	panic("unexpected GenerateAccountID call")
}

func (NoopRuntimeInterface) RecordTrace(_ string, _ runtime.Location, _ time.Duration, _ []attribute.KeyValue) {
	panic("unexpected RecordTrace call")
}
