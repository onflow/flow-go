package migrations

import (
	"fmt"
	"strings"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
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
	log zerolog.Logger,
	verboseLogging bool,
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
		err := validateStorageDomain(address, oldRuntime, newRuntime, domain, log, verboseLogging)
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
	log zerolog.Logger,
	verboseLogging bool,
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

		err := cadenceValueEqual(oldRuntime.Interpreter, oldValue, newRuntime.Interpreter, newValue)
		if err != nil {
			if verboseLogging {
				log.Info().
					Str("address", address.Hex()).
					Str("domain", domain).
					Str("key", string(stringKey)).
					Str("trace", err.Error()).
					Str("old value", oldValue.String()).
					Str("new value", newValue.String()).
					Msgf("failed to validate value")
			}

			return fmt.Errorf("failed to validate value for address %s, domain %s, key %s: %s", address.Hex(), domain, key, err.Error())
		}
	}

	return nil
}

type validationError struct {
	trace         []string
	errorMsg      string
	traceReversed bool
}

func newValidationErrorf(format string, a ...any) *validationError {
	return &validationError{
		errorMsg: fmt.Sprintf(format, a...),
	}
}

func (e *validationError) addTrace(trace string) {
	e.trace = append(e.trace, trace)
}

func (e *validationError) Error() string {
	if len(e.trace) == 0 {
		return fmt.Sprintf("failed to validate: %s", e.errorMsg)
	}
	// Reverse trace
	if !e.traceReversed {
		for i, j := 0, len(e.trace)-1; i < j; i, j = i+1, j-1 {
			e.trace[i], e.trace[j] = e.trace[j], e.trace[i]
		}
		e.traceReversed = true
	}
	trace := strings.Join(e.trace, ".")
	return fmt.Sprintf("failed to validate %s: %s", trace, e.errorMsg)
}

func cadenceValueEqual(
	vInterpreter *interpreter.Interpreter,
	v interpreter.Value,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) *validationError {
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
			return newValidationErrorf(
				"value doesn't implement interpreter.EquatableValue: %T",
				oldValue,
			)
		}
		if !oldValue.Equal(nil, interpreter.EmptyLocationRange, other) {
			return newValidationErrorf(
				"values differ: %v (%T) != %v (%T)",
				oldValue,
				oldValue,
				other,
				other,
			)
		}
	}

	return nil
}

func cadenceSomeValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.SomeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) *validationError {
	otherSome, ok := other.(*interpreter.SomeValue)
	if !ok {
		return newValidationErrorf("types differ: %T != %T", v, other)
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
) *validationError {
	otherArray, ok := other.(*interpreter.ArrayValue)
	if !ok {
		return newValidationErrorf("types differ: %T != %T", v, other)
	}

	count := v.Count()
	if count != otherArray.Count() {
		return newValidationErrorf("array counts differ: %d != %d", count, otherArray.Count())
	}

	if v.Type == nil {
		if otherArray.Type != nil {
			return newValidationErrorf("array types differ: nil != %s", otherArray.Type)
		}
	} else { // v.Type != nil
		if otherArray.Type == nil {
			return newValidationErrorf("array types differ: %s != nil", v.Type)
		} else if !v.Type.Equal(otherArray.Type) {
			return newValidationErrorf("array types differ: %s != %s", v.Type, otherArray.Type)
		}
	}

	for i := 0; i < count; i++ {
		element := v.Get(vInterpreter, interpreter.EmptyLocationRange, i)
		otherElement := otherArray.Get(otherInterpreter, interpreter.EmptyLocationRange, i)

		err := cadenceValueEqual(vInterpreter, element, otherInterpreter, otherElement)
		if err != nil {
			err.addTrace(fmt.Sprintf("(%s[%d])", v.Type, i))
			return err
		}
	}

	return nil
}

func cadenceCompositeValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.CompositeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) *validationError {
	otherComposite, ok := other.(*interpreter.CompositeValue)
	if !ok {
		return newValidationErrorf("types differ: %T != %T", v, other)
	}

	if !v.StaticType(vInterpreter).Equal(otherComposite.StaticType(otherInterpreter)) {
		return newValidationErrorf(
			"composite types differ: %s != %s",
			v.StaticType(vInterpreter),
			otherComposite.StaticType(otherInterpreter),
		)
	}

	if v.Kind != otherComposite.Kind {
		return newValidationErrorf(
			"composite kinds differ: %d != %d",
			v.Kind,
			otherComposite.Kind,
		)
	}

	var err *validationError
	vFieldNames := make([]string, 0, 10) // v's field names
	v.ForEachField(nopMemoryGauge, func(fieldName string, fieldValue interpreter.Value) bool {
		otherFieldValue := otherComposite.GetField(otherInterpreter, interpreter.EmptyLocationRange, fieldName)

		err = cadenceValueEqual(vInterpreter, fieldValue, otherInterpreter, otherFieldValue)
		if err != nil {
			err.addTrace(fmt.Sprintf("(%s.%s)", v.TypeID(), fieldName))
			return false
		}

		vFieldNames = append(vFieldNames, fieldName)
		return true
	})

	// TODO: Use CompositeValue.FieldCount() from Cadence after it is merged and available.
	otherFieldNames := make([]string, 0, len(vFieldNames)) // otherComposite's field names
	otherComposite.ForEachField(nopMemoryGauge, func(fieldName string, _ interpreter.Value) bool {
		otherFieldNames = append(otherFieldNames, fieldName)
		return true
	})

	if len(vFieldNames) != len(otherFieldNames) {
		return newValidationErrorf(
			"composite %s fields differ: %v != %v",
			v.TypeID(),
			vFieldNames,
			otherFieldNames,
		)
	}

	return err
}

func cadenceDictionaryValueEqual(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.DictionaryValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
) *validationError {
	otherDictionary, ok := other.(*interpreter.DictionaryValue)
	if !ok {
		return newValidationErrorf("types differ: %T != %T", v, other)
	}

	if v.Count() != otherDictionary.Count() {
		return newValidationErrorf("dict counts differ: %d != %d", v.Count(), otherDictionary.Count())
	}

	if !v.Type.Equal(otherDictionary.Type) {
		return newValidationErrorf("dict types differ: %s != %s", v.Type, otherDictionary.Type)
	}

	oldIterator := v.Iterator()
	for {
		key := oldIterator.NextKey(nopMemoryGauge)
		if key == nil {
			break
		}

		oldValue, oldValueExist := v.Get(vInterpreter, interpreter.EmptyLocationRange, key)
		if !oldValueExist {
			err := newValidationErrorf("old value doesn't exist with key %v (%T)", key, key)
			err.addTrace(fmt.Sprintf("(%s[%s])", v.Type, key))
			return err
		}
		newValue, newValueExist := otherDictionary.Get(otherInterpreter, interpreter.EmptyLocationRange, key)
		if !newValueExist {
			err := newValidationErrorf("new value doesn't exist with key %v (%T)", key, key)
			err.addTrace(fmt.Sprintf("(%s[%s])", otherDictionary.Type, key))
			return err
		}
		err := cadenceValueEqual(vInterpreter, oldValue, otherInterpreter, newValue)
		if err != nil {
			err.addTrace(fmt.Sprintf("(%s[%s])", otherDictionary.Type, key))
			return err
		}
	}

	return nil
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
		// Attachments are enabled everywhere except for Mainnet
		AttachmentsEnabled: true,
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

func (NoopRuntimeInterface) AllocateSlabIndex(_ []byte) (atree.SlabIndex, error) {
	panic("unexpected AllocateSlabIndex call")
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
