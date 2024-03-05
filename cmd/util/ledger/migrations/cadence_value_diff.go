package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
)

type diffKind int

const (
	storageMapExistDiffKind        diffKind = iota // Storage map only exists in one state
	storageMapKeyDiffKind                          // Storage map keys are different
	storageMapValueDiffKind                        // Storage map values are different (only with verbose logging)
	cadenceValueDiffKind                           // Cadence values are different
	cadenceValueTypeDiffKind                       // Cadence value types are different
	cadenceValueStaticTypeDiffKind                 // Cadence value static types are different
)

var diffKindString = map[diffKind]string{
	storageMapExistDiffKind:        "storage_map_exist_diff",
	storageMapKeyDiffKind:          "storage_map_key_diff",
	storageMapValueDiffKind:        "storage_map_value_diff",
	cadenceValueDiffKind:           "cadence_value_diff",
	cadenceValueTypeDiffKind:       "cadence_value_type_diff",
	cadenceValueStaticTypeDiffKind: "cadence_value_static_type_diff",
}

type diffErrorKind int

const (
	storageMapKeyNotImplementingStorageMapKeyDiffErrorKind diffErrorKind = iota
	cadenceValueNotImplementEquatableValueDiffErrorKind
)

var diffErrorKindString = map[diffErrorKind]string{
	storageMapKeyNotImplementingStorageMapKeyDiffErrorKind: "error_storage_map_key_not_implemeting_StorageMapKey",
	cadenceValueNotImplementEquatableValueDiffErrorKind:    "error_cadence_value_not_implementing_EquatableValue",
}

type diffProblem struct {
	Address string
	Domain  string
	Kind    string
	Msg     string
	Trace   string `json:",omitempty"`
}

type difference struct {
	Address            string
	Domain             string
	Kind               string
	Msg                string
	Trace              string `json:",omitempty"`
	OldValue           string `json:",omitempty"`
	NewValue           string `json:",omitempty"`
	OldValueStaticType string `json:",omitempty"`
	NewValueStaticType string `json:",omitempty"`
}

type CadenceValueDiffReporter struct {
	address        common.Address
	reportWriter   reporters.ReportWriter
	verboseLogging bool
}

func NewCadenceValueDiffReporter(
	address common.Address,
	rw reporters.ReportWriter,
	verboseLogging bool,
) *CadenceValueDiffReporter {
	return &CadenceValueDiffReporter{
		address:        address,
		reportWriter:   rw,
		verboseLogging: verboseLogging,
	}
}

func (dr *CadenceValueDiffReporter) DiffStates(oldPayloads, newPayloads []*ledger.Payload, domains []string) error {
	// Create all the runtime components we need for comparing Cadence values.
	oldRuntime, err := newReadonlyStorageRuntime(oldPayloads)
	if err != nil {
		return fmt.Errorf("failed to create runtime with old state payloads: %w", err)
	}

	newRuntime, err := newReadonlyStorageRuntime(newPayloads)
	if err != nil {
		return fmt.Errorf("failed to create runtime with new state payloads: %w", err)
	}

	// Iterate through all domains and compare cadence values.
	for _, domain := range domains {
		err := dr.diffStorageDomain(oldRuntime, newRuntime, domain)
		if err != nil {
			return err
		}
	}

	return nil
}

func (dr *CadenceValueDiffReporter) diffStorageDomain(oldRuntime, newRuntime *readonlyStorageRuntime, domain string) error {

	oldStorageMap := oldRuntime.Storage.GetStorageMap(dr.address, domain, false)

	newStorageMap := newRuntime.Storage.GetStorageMap(dr.address, domain, false)

	if oldStorageMap == nil && newStorageMap == nil {
		// No storage maps for this domain.
		return nil
	}

	if oldStorageMap == nil && newStorageMap != nil {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[storageMapExistDiffKind],
				Msg: fmt.Sprintf(
					"old storage map doesn't exist, new storage map has %d elements with keys %v",
					newStorageMap.Count(),
					getStorageMapKeys(newStorageMap),
				),
			})

		return nil
	}

	if oldStorageMap != nil && newStorageMap == nil {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[storageMapExistDiffKind],
				Msg: fmt.Sprintf(
					"new storage map doesn't exist, old storage map has %d elements with keys %v",
					oldStorageMap.Count(),
					getStorageMapKeys(oldStorageMap),
				),
			})

		return nil
	}

	oldKeys := getStorageMapKeys(oldStorageMap)
	newKeys := getStorageMapKeys(newStorageMap)

	onlyOldKeys, onlyNewKeys, sharedKeys := diff(oldKeys, newKeys)

	// Log keys only present in old storage map
	if len(onlyOldKeys) > 0 {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[storageMapKeyDiffKind],
				Msg: fmt.Sprintf(
					"old storage map has %d elements with keys %v, that are not present in new storge map",
					len(onlyOldKeys),
					onlyOldKeys,
				),
			})
	}

	// Log keys only present in new storage map
	if len(onlyNewKeys) > 0 {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[storageMapKeyDiffKind],
				Msg: fmt.Sprintf(
					"new storage map has %d elements with keys %v, that are not present in old storge map",
					len(onlyNewKeys),
					onlyNewKeys,
				),
			})
	}

	// Compare elements present in both storage maps
	for _, key := range sharedKeys {

		trace := fmt.Sprintf("%s[%v]", domain, key)

		var mapKey interpreter.StorageMapKey

		switch key := key.(type) {
		case interpreter.StringAtreeValue:
			mapKey = interpreter.StringStorageMapKey(key)

		case interpreter.Uint64AtreeValue:
			mapKey = interpreter.Uint64StorageMapKey(key)

		case interpreter.StringStorageMapKey:
			mapKey = key

		case interpreter.Uint64StorageMapKey:
			mapKey = key

		default:
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain,
					Kind:    diffErrorKindString[storageMapKeyNotImplementingStorageMapKeyDiffErrorKind],
					Trace:   trace,
					Msg: fmt.Sprintf(
						"invalid storage map key %v (%T), expected interpreter.StorageMapKey",
						key,
						key,
					),
				})
			continue
		}

		oldValue := oldStorageMap.ReadValue(nopMemoryGauge, mapKey)

		newValue := newStorageMap.ReadValue(nopMemoryGauge, mapKey)

		hasDifference := dr.diffValues(
			oldRuntime.Interpreter,
			oldValue,
			newRuntime.Interpreter,
			newValue,
			domain,
			trace,
		)
		if hasDifference {
			if dr.verboseLogging {
				// Log potentially large values at top level only when verbose logging is enabled.
				dr.reportWriter.Write(
					difference{
						Address:            dr.address.Hex(),
						Domain:             domain,
						Kind:               diffKindString[storageMapValueDiffKind],
						Msg:                "storage map elements are different",
						Trace:              trace,
						OldValue:           oldValue.String(),
						NewValue:           newValue.String(),
						OldValueStaticType: oldValue.StaticType(oldRuntime.Interpreter).String(),
						NewValueStaticType: newValue.StaticType(newRuntime.Interpreter).String(),
					})
			}
		}

	}

	return nil
}

func (dr *CadenceValueDiffReporter) diffValues(
	vInterpreter *interpreter.Interpreter,
	v interpreter.Value,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain string,
	trace string,
) (hasDifference bool) {
	switch v := v.(type) {
	case *interpreter.ArrayValue:
		return dr.diffCadenceArrayValue(vInterpreter, v, otherInterpreter, other, domain, trace)

	case *interpreter.CompositeValue:
		return dr.diffCadenceCompositeValue(vInterpreter, v, otherInterpreter, other, domain, trace)

	case *interpreter.DictionaryValue:
		return dr.diffCadenceDictionaryValue(vInterpreter, v, otherInterpreter, other, domain, trace)

	case *interpreter.SomeValue:
		return dr.diffCadenceSomeValue(vInterpreter, v, otherInterpreter, other, domain, trace)

	default:
		oldValue, ok := v.(interpreter.EquatableValue)
		if !ok {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain,
					Kind:    diffErrorKindString[cadenceValueNotImplementEquatableValueDiffErrorKind],
					Trace:   trace,
					Msg:     fmt.Sprintf("old value doesn't implement interpreter.EquatableValue: %s (%T)", oldValue.String(), oldValue),
				})
			return true
		}

		if !oldValue.Equal(nil, interpreter.EmptyLocationRange, other) {
			dr.reportWriter.Write(
				difference{
					Address:            dr.address.Hex(),
					Domain:             domain,
					Kind:               diffKindString[cadenceValueDiffKind],
					Msg:                fmt.Sprintf("values differ: %T vs %T", oldValue, other),
					Trace:              trace,
					OldValue:           v.String(),
					NewValue:           other.String(),
					OldValueStaticType: v.StaticType(vInterpreter).String(),
					NewValueStaticType: other.StaticType(otherInterpreter).String(),
				})
			return true
		}
	}

	return false
}

func (dr *CadenceValueDiffReporter) diffCadenceSomeValue(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.SomeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain string,
	trace string,
) (hasDifference bool) {
	otherSome, ok := other.(*interpreter.SomeValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	innerValue := v.InnerValue(vInterpreter, interpreter.EmptyLocationRange)

	otherInnerValue := otherSome.InnerValue(otherInterpreter, interpreter.EmptyLocationRange)

	return dr.diffValues(vInterpreter, innerValue, otherInterpreter, otherInnerValue, domain, trace)
}

func (dr *CadenceValueDiffReporter) diffCadenceArrayValue(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.ArrayValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain string,
	trace string,
) (hasDifference bool) {
	otherArray, ok := other.(*interpreter.ArrayValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	if v.Type == nil && otherArray.Type != nil {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("array static types differ: nil != %s", otherArray.Type),
			})
	}

	if v.Type != nil && otherArray.Type == nil {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("array static types differ: %s != nil", v.Type),
			})
	}

	if v.Type != nil && otherArray.Type != nil && !v.Type.Equal(otherArray.Type) {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("array static types differ: %s != %s", v.Type, otherArray.Type),
			})
	}

	count := v.Count()
	if count != otherArray.Count() {
		hasDifference = true

		d := difference{
			Address: dr.address.Hex(),
			Domain:  domain,
			Kind:    diffKindString[cadenceValueDiffKind],
			Trace:   trace,
			Msg:     fmt.Sprintf("array counts differ: %d != %d", count, otherArray.Count()),
		}

		if dr.verboseLogging {
			d.OldValue = v.String()
			d.NewValue = other.String()
		}

		dr.reportWriter.Write(d)
	}

	// Compare array elements
	for i := 0; i < min(count, otherArray.Count()); i++ {
		element := v.Get(vInterpreter, interpreter.EmptyLocationRange, i)
		otherElement := otherArray.Get(otherInterpreter, interpreter.EmptyLocationRange, i)

		elementTrace := fmt.Sprintf("%s[%d]", trace, i)
		elementHasDifference := dr.diffValues(vInterpreter, element, otherInterpreter, otherElement, domain, elementTrace)
		if elementHasDifference {
			hasDifference = true
		}
	}

	return hasDifference
}

func (dr *CadenceValueDiffReporter) diffCadenceCompositeValue(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.CompositeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain string,
	trace string,
) (hasDifference bool) {
	otherComposite, ok := other.(*interpreter.CompositeValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	if !v.StaticType(vInterpreter).Equal(otherComposite.StaticType(otherInterpreter)) {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"composite static types differ: %s != %s",
					v.StaticType(vInterpreter),
					otherComposite.StaticType(otherInterpreter)),
			})
	}

	if v.Kind != otherComposite.Kind {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"composite kinds differ: %d != %d",
					v.Kind,
					otherComposite.Kind,
				),
			})
	}

	oldFieldNames := make([]string, 0, v.FieldCount())
	v.ForEachFieldName(func(fieldName string) bool {
		oldFieldNames = append(oldFieldNames, fieldName)
		return true
	})

	newFieldNames := make([]string, 0, otherComposite.FieldCount())
	otherComposite.ForEachFieldName(func(fieldName string) bool {
		newFieldNames = append(newFieldNames, fieldName)
		return true
	})

	onlyOldFieldNames, onlyNewFieldNames, sharedFieldNames := diff(oldFieldNames, newFieldNames)

	// Log field names only present in old composite value
	if len(onlyOldFieldNames) > 0 {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"old composite value has %d fields with keys %v, that are not present in new composite value",
					len(onlyOldFieldNames),
					onlyOldFieldNames,
				),
			})
	}

	// Log field names only present in new composite value
	if len(onlyNewFieldNames) > 0 {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"new composite value has %d fields with keys %v, that are not present in old composite value",
					len(onlyNewFieldNames),
					onlyNewFieldNames,
				),
			})
	}

	// Compare fields in both composite values
	for _, fieldName := range sharedFieldNames {
		fieldValue := v.GetField(vInterpreter, interpreter.EmptyLocationRange, fieldName)
		otherFieldValue := otherComposite.GetField(otherInterpreter, interpreter.EmptyLocationRange, fieldName)

		fieldTrace := fmt.Sprintf("%s.%s", trace, fieldName)
		fieldHasDifference := dr.diffValues(vInterpreter, fieldValue, otherInterpreter, otherFieldValue, domain, fieldTrace)
		if fieldHasDifference {
			hasDifference = true
		}
	}

	return hasDifference
}

func (dr *CadenceValueDiffReporter) diffCadenceDictionaryValue(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.DictionaryValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain string,
	trace string,
) (hasDifference bool) {
	otherDictionary, ok := other.(*interpreter.DictionaryValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace,
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	if !v.Type.Equal(otherDictionary.Type) {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"dict static types differ: %s != %s",
					v.Type,
					otherDictionary.Type),
			})
	}

	oldKeys := make([]interpreter.Value, 0, v.Count())
	v.IterateKeys(vInterpreter, func(key interpreter.Value) (resume bool) {
		oldKeys = append(oldKeys, key)
		return true
	})

	newKeys := make([]interpreter.Value, 0, otherDictionary.Count())
	otherDictionary.IterateKeys(otherInterpreter, func(key interpreter.Value) (resume bool) {
		newKeys = append(newKeys, key)
		return true
	})

	onlyOldKeys, onlyNewKeys, sharedKeys := diffCadenceValues(oldKeys, newKeys)

	// Log keys only present in old dict value
	if len(onlyOldKeys) > 0 {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"old dict value has %d elements with keys %v, that are not present in new dict value",
					len(onlyOldKeys),
					onlyOldKeys,
				),
			})
	}

	// Log field names only present in new composite value
	if len(onlyNewKeys) > 0 {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain,
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace,
				Msg: fmt.Sprintf(
					"new dict value has %d elements with keys %v, that are not present in old dict value",
					len(onlyNewKeys),
					onlyNewKeys,
				),
			})
	}

	// Compare elements in both dict values
	for _, key := range sharedKeys {
		valueTrace := fmt.Sprintf("%s[%v]", trace, key)

		oldValue, _ := v.Get(vInterpreter, interpreter.EmptyLocationRange, key)

		newValue, _ := otherDictionary.Get(otherInterpreter, interpreter.EmptyLocationRange, key)

		elementHasDifference := dr.diffValues(vInterpreter, oldValue, otherInterpreter, newValue, domain, valueTrace)
		if elementHasDifference {
			hasDifference = true
		}
	}

	return hasDifference
}

func getStorageMapKeys(storageMap *interpreter.StorageMap) []any {
	keys := make([]any, 0, storageMap.Count())

	iter := storageMap.Iterator(nil)
	for {
		key := iter.NextKey()
		if key == nil {
			break
		}
		keys = append(keys, key)
	}

	return keys
}

func diff[T comparable](old, new []T) (onlyOld, onlyNew, shared []T) {
	onlyOld = make([]T, 0, len(old))
	onlyNew = make([]T, 0, len(new))
	shared = make([]T, 0, min(len(old), len(new)))

	sharedNew := make([]bool, len(new))

	for _, o := range old {
		found := false

		for i, n := range new {
			if o == n {
				shared = append(shared, o)
				found = true
				sharedNew[i] = true
				break
			}
		}

		if !found {
			onlyOld = append(onlyOld, o)
		}
	}

	for i, shared := range sharedNew {
		if !shared {
			onlyNew = append(onlyNew, new[i])
		}
	}

	return
}

func diffCadenceValues(old, new []interpreter.Value) (onlyOld, onlyNew, shared []interpreter.Value) {
	onlyOld = make([]interpreter.Value, 0, len(old))
	onlyNew = make([]interpreter.Value, 0, len(new))
	shared = make([]interpreter.Value, 0, min(len(old), len(new)))

	sharedNew := make([]bool, len(new))

	for _, o := range old {
		found := false

		for i, n := range new {
			foundShared := false

			if ev, ok := o.(interpreter.EquatableValue); ok {
				if ev.Equal(nil, interpreter.EmptyLocationRange, n) {
					foundShared = true
				}
			} else {
				if o == n {
					foundShared = true
				}
			}

			if foundShared {
				shared = append(shared, o)
				found = true
				sharedNew[i] = true
				break
			}
		}

		if !found {
			onlyOld = append(onlyOld, o)
		}
	}

	for i, shared := range sharedNew {
		if !shared {
			onlyNew = append(onlyNew, new[i])
		}
	}

	return
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}
