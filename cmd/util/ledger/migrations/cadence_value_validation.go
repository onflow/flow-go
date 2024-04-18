package migrations

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
)

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
	for _, domain := range allStorageMapDomains {
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

	oldIterator := oldStorageMap.Iterator(nil)
	for {
		key, oldValue := oldIterator.Next()
		if key == nil {
			break
		}

		var mapKey interpreter.StorageMapKey

		switch key := key.(type) {
		case interpreter.StringAtreeValue:
			mapKey = interpreter.StringStorageMapKey(key)

		case interpreter.Uint64AtreeValue:
			mapKey = interpreter.Uint64StorageMapKey(key)

		default:
			return fmt.Errorf("invalid key type %T, expected interpreter.StringAtreeValue or interpreter.Uint64AtreeValue", key)
		}

		newValue := newStorageMap.ReadValue(nil, mapKey)

		err := cadenceValueEqual(
			oldRuntime.Interpreter,
			oldValue,
			newRuntime.Interpreter,
			newValue,
		)
		if err != nil {
			if verboseLogging {
				log.Info().
					Str("address", address.Hex()).
					Str("domain", domain).
					Str("key", fmt.Sprintf("%v (%T)", mapKey, mapKey)).
					Str("trace", err.Error()).
					Str("old value", oldValue.String()).
					Str("new value", newValue.String()).
					Msgf("failed to validate value")
			}

			return fmt.Errorf(
				"failed to validate value for address %s, domain %s, key %s: %s",
				address.Hex(),
				domain,
				key,
				err.Error(),
			)
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
	v.ForEachField(nil, func(fieldName string, fieldValue interpreter.Value) bool {
		otherFieldValue := otherComposite.GetField(otherInterpreter, interpreter.EmptyLocationRange, fieldName)

		err = cadenceValueEqual(vInterpreter, fieldValue, otherInterpreter, otherFieldValue)
		if err != nil {
			err.addTrace(fmt.Sprintf("(%s.%s)", v.TypeID(), fieldName))
			return false
		}

		vFieldNames = append(vFieldNames, fieldName)
		return true
	})
	if err != nil {
		return err
	}

	if len(vFieldNames) != otherComposite.FieldCount() {
		otherFieldNames := make([]string, 0, len(vFieldNames)) // otherComposite's field names
		otherComposite.ForEachField(nil, func(fieldName string, _ interpreter.Value) bool {
			otherFieldNames = append(otherFieldNames, fieldName)
			return true
		})

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
		key := oldIterator.NextKey(nil)
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

	storage := runtime.NewStorage(readonlyLedger, nil)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	if err != nil {
		return nil, err
	}

	return &readonlyStorageRuntime{
		Interpreter: inter,
		Storage:     storage,
	}, nil
}
