package migrations

import (
	"fmt"
	"time"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
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
	abortErrorKind diffErrorKind = iota
	storageMapKeyNotImplementingStorageMapKeyDiffErrorKind
	cadenceValueNotImplementEquatableValueDiffErrorKind
)

var diffErrorKindString = map[diffErrorKind]string{
	abortErrorKind: "error_diff_failed",
	storageMapKeyNotImplementingStorageMapKeyDiffErrorKind: "error_storage_map_key_not_implementing_StorageMapKey",
	cadenceValueNotImplementEquatableValueDiffErrorKind:    "error_cadence_value_not_implementing_EquatableValue",
}

type diffError struct {
	Address string
	Kind    string
	Msg     string
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

const minLargeAccountRegisterCount = 1_000_000

type CadenceValueDiffReporter struct {
	address        common.Address
	chainID        flow.ChainID
	reportWriter   reporters.ReportWriter
	verboseLogging bool
	nWorkers       int
}

func NewCadenceValueDiffReporter(
	address common.Address,
	chainID flow.ChainID,
	rw reporters.ReportWriter,
	verboseLogging bool,
	nWorkers int,
) *CadenceValueDiffReporter {
	return &CadenceValueDiffReporter{
		address:        address,
		chainID:        chainID,
		reportWriter:   rw,
		verboseLogging: verboseLogging,
		nWorkers:       nWorkers,
	}
}

type IsValueIncludedFunc func(address common.Address, domain common.StorageDomain, key any) bool

func (dr *CadenceValueDiffReporter) DiffStates(
	oldRegs, newRegs registers.Registers,
	domains []common.StorageDomain,
	isValueIncluded IsValueIncludedFunc,
) {

	oldStorage := newReadonlyStorage(oldRegs)

	newStorage := newReadonlyStorage(newRegs)

	var loadAtreeStorageGroup errgroup.Group

	loadAtreeStorageGroup.Go(func() (err error) {
		return util.LoadAtreeSlabsInStorage(oldStorage, oldRegs, dr.nWorkers)
	})

	err := util.LoadAtreeSlabsInStorage(newStorage, newRegs, dr.nWorkers)
	if err != nil {
		dr.reportWriter.Write(
			diffError{
				Address: dr.address.Hex(),
				Kind:    diffErrorKindString[abortErrorKind],
				Msg:     fmt.Sprintf("failed to preload new atree registers: %s", err),
			})
		return
	}

	// Wait for old registers to be loaded in storage.
	if err := loadAtreeStorageGroup.Wait(); err != nil {
		dr.reportWriter.Write(
			diffError{
				Address: dr.address.Hex(),
				Kind:    diffErrorKindString[abortErrorKind],
				Msg:     fmt.Sprintf("failed to preload old atree registers: %s", err),
			})
		return
	}

	// Skip goroutine overhead for smaller accounts
	oldRuntime, err := newReadonlyStorageRuntimeWithStorage(oldStorage, oldRegs.Count())
	if err != nil {
		dr.reportWriter.Write(
			diffError{
				Address: dr.address.Hex(),
				Kind:    diffErrorKindString[abortErrorKind],
				Msg:     fmt.Sprintf("failed to create runtime for old registers: %s", err),
			})
		return
	}

	newRuntime, err := newReadonlyStorageRuntimeWithStorage(newStorage, newRegs.Count())
	if err != nil {
		dr.reportWriter.Write(
			diffError{
				Address: dr.address.Hex(),
				Kind:    diffErrorKindString[abortErrorKind],
				Msg:     fmt.Sprintf("failed to create runtime with new registers: %s", err),
			})
		return
	}

	for _, domain := range domains {
		dr.diffDomain(oldRuntime, newRuntime, domain, isValueIncluded)
	}
}

func (dr *CadenceValueDiffReporter) diffDomain(
	oldRuntime *readonlyStorageRuntime,
	newRuntime *readonlyStorageRuntime,
	domain common.StorageDomain,
	isValueIncluded IsValueIncludedFunc,
) {
	defer func() {
		if r := recover(); r != nil {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[abortErrorKind],
					Msg: fmt.Sprintf(
						"panic while diffing storage maps: %s",
						r,
					),
				},
			)
		}
	}()

	oldStorageMap := oldRuntime.Storage.GetDomainStorageMap(oldRuntime.Interpreter, dr.address, domain, false)
	newStorageMap := newRuntime.Storage.GetDomainStorageMap(newRuntime.Interpreter, dr.address, domain, false)

	if oldStorageMap == nil && newStorageMap == nil {
		// No storage maps for this domain.
		return
	}

	if oldStorageMap == nil && newStorageMap != nil {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[storageMapExistDiffKind],
				Msg: fmt.Sprintf(
					"old storage map doesn't exist, new storage map has %d elements with keys %v",
					newStorageMap.Count(),
					getStorageMapKeys(newStorageMap),
				),
			})

		return
	}

	if oldStorageMap != nil && newStorageMap == nil {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[storageMapExistDiffKind],
				Msg: fmt.Sprintf(
					"new storage map doesn't exist, old storage map has %d elements with keys %v",
					oldStorageMap.Count(),
					getStorageMapKeys(oldStorageMap),
				),
			})

		return
	}

	oldKeys := getStorageMapKeys(oldStorageMap)
	newKeys := getStorageMapKeys(newStorageMap)

	onlyOldKeys, onlyNewKeys, sharedKeys := diff(oldKeys, newKeys)

	// Log keys only present in old storage map
	if len(onlyOldKeys) > 0 {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
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
				Domain:  domain.Identifier(),
				Kind:    diffKindString[storageMapKeyDiffKind],
				Msg: fmt.Sprintf(
					"new storage map has %d elements with keys %v, that are not present in old storge map",
					len(onlyNewKeys),
					onlyNewKeys,
				),
			})
	}

	if len(sharedKeys) == 0 {
		return
	}

	getValues := func(key any) (interpreter.Value, interpreter.Value, *util.Trace, bool) {

		trace := util.NewTrace(fmt.Sprintf("%s[%v]", domain.Identifier(), key))

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
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[storageMapKeyNotImplementingStorageMapKeyDiffErrorKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"invalid storage map key %v (%T), expected interpreter.StorageMapKey",
						key,
						key,
					),
				})
			return nil, nil, nil, false
		}

		oldValue := oldStorageMap.ReadValue(nil, mapKey)

		newValue := newStorageMap.ReadValue(nil, mapKey)

		return oldValue, newValue, trace, true
	}

	diffValues := func(
		oldInterpreter *interpreter.Interpreter,
		oldValue interpreter.Value,
		newInterpreter *interpreter.Interpreter,
		newValue interpreter.Value,
		trace *util.Trace,
	) {
		hasDifference := dr.diffValues(
			oldInterpreter,
			oldValue,
			newInterpreter,
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
						Domain:             domain.Identifier(),
						Kind:               diffKindString[storageMapValueDiffKind],
						Msg:                "storage map elements are different",
						Trace:              trace.String(),
						OldValue:           oldValue.String(),
						NewValue:           newValue.String(),
						OldValueStaticType: oldValue.StaticType(oldInterpreter).String(),
						NewValueStaticType: newValue.StaticType(newInterpreter).String(),
					})
			}
		}
	}

	startTime := time.Now()

	isLargeAccount := oldRuntime.PayloadCount > minLargeAccountRegisterCount

	if isLargeAccount {
		log.Info().Msgf(
			"Diffing %x storage domain containing %d elements (%d payloads) ...",
			dr.address[:],
			len(sharedKeys),
			oldRuntime.PayloadCount,
		)
	}

	// Diffing storage domain

	for _, key := range sharedKeys {
		if !isValueIncluded(dr.address, domain, key) {
			continue
		}
		oldValue, newValue, trace, canDiff := getValues(key)
		if canDiff {
			diffValues(
				oldRuntime.Interpreter,
				oldValue,
				newRuntime.Interpreter,
				newValue,
				trace,
			)
		}
	}

	if isLargeAccount {
		log.Info().
			Msgf(
				"Finished diffing %x storage domain containing %d elements (%d payloads) in %s",
				dr.address[:],
				len(sharedKeys),
				oldRuntime.PayloadCount,
				time.Since(startTime),
			)
	}
}

func (dr *CadenceValueDiffReporter) diffValues(
	vInterpreter *interpreter.Interpreter,
	v interpreter.Value,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain common.StorageDomain,
	trace *util.Trace,
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
		return dr.diffEquatable(vInterpreter, v, otherInterpreter, other, domain, trace)
	}
}

func (dr *CadenceValueDiffReporter) diffEquatable(
	vInterpreter *interpreter.Interpreter,
	v interpreter.Value,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain common.StorageDomain,
	trace *util.Trace,
) (hasDifference bool) {

	defer func() {
		if r := recover(); r != nil {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[abortErrorKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"panic while diffing values: %s",
						r,
					),
				},
			)
		}
	}()

	oldValue, ok := v.(interpreter.EquatableValue)
	if !ok {
		dr.reportWriter.Write(
			diffProblem{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffErrorKindString[cadenceValueNotImplementEquatableValueDiffErrorKind],
				Trace:   trace.String(),
				Msg: fmt.Sprintf(
					"old value doesn't implement interpreter.EquatableValue: %s (%T)",
					oldValue.String(),
					oldValue,
				),
			})
		return true
	}

	if !oldValue.Equal(nil, interpreter.EmptyLocationRange, other) {
		dr.reportWriter.Write(
			difference{
				Address:            dr.address.Hex(),
				Domain:             domain.Identifier(),
				Kind:               diffKindString[cadenceValueDiffKind],
				Msg:                fmt.Sprintf("values differ: %T vs %T", oldValue, other),
				Trace:              trace.String(),
				OldValue:           v.String(),
				NewValue:           other.String(),
				OldValueStaticType: v.StaticType(vInterpreter).String(),
				NewValueStaticType: other.StaticType(otherInterpreter).String(),
			})
		return true
	}

	return false
}

func (dr *CadenceValueDiffReporter) diffCadenceSomeValue(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.SomeValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain common.StorageDomain,
	trace *util.Trace,
) (hasDifference bool) {

	defer func() {
		if r := recover(); r != nil {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[abortErrorKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"panic while diffing some: %s",
						r,
					),
				},
			)
		}
	}()

	otherSome, ok := other.(*interpreter.SomeValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	innerValue := v.InnerValue()

	otherInnerValue := otherSome.InnerValue()

	return dr.diffValues(
		vInterpreter,
		innerValue,
		otherInterpreter,
		otherInnerValue,
		domain,
		trace,
	)
}

func (dr *CadenceValueDiffReporter) diffCadenceArrayValue(
	vInterpreter *interpreter.Interpreter,
	v *interpreter.ArrayValue,
	otherInterpreter *interpreter.Interpreter,
	other interpreter.Value,
	domain common.StorageDomain,
	trace *util.Trace,
) (hasDifference bool) {

	defer func() {
		if r := recover(); r != nil {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[abortErrorKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"panic while diffing array: %s",
						r,
					),
				},
			)
		}
	}()

	otherArray, ok := other.(*interpreter.ArrayValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	if v.Type == nil && otherArray.Type != nil {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("array static types differ: nil != %s", otherArray.Type),
			})
	}

	if v.Type != nil && otherArray.Type == nil {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("array static types differ: %s != nil", v.Type),
			})
	}

	if v.Type != nil && otherArray.Type != nil && !v.Type.Equal(otherArray.Type) {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("array static types differ: %s != %s", v.Type, otherArray.Type),
			})
	}

	count := v.Count()
	if count != otherArray.Count() {
		hasDifference = true

		d := difference{
			Address: dr.address.Hex(),
			Domain:  domain.Identifier(),
			Kind:    diffKindString[cadenceValueDiffKind],
			Trace:   trace.String(),
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

		elementTrace := trace.Append(fmt.Sprintf("[%d]", i))
		elementHasDifference := dr.diffValues(
			vInterpreter,
			element,
			otherInterpreter,
			otherElement,
			domain,
			elementTrace,
		)
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
	domain common.StorageDomain,
	trace *util.Trace,
) (hasDifference bool) {

	defer func() {
		if r := recover(); r != nil {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[abortErrorKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"panic while diffing composite: %s",
						r,
					),
				},
			)
		}
	}()

	otherComposite, ok := other.(*interpreter.CompositeValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	if !v.StaticType(vInterpreter).Equal(otherComposite.StaticType(otherInterpreter)) {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace.String(),
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
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace.String(),
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
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace.String(),
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
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace.String(),
				Msg: fmt.Sprintf(
					"new composite value has %d fields with keys %v, that are not present in old composite value",
					len(onlyNewFieldNames),
					onlyNewFieldNames,
				),
			})
	}

	// Compare fields in both composite values
	for _, fieldName := range sharedFieldNames {
		fieldValue := v.GetField(vInterpreter, fieldName)
		otherFieldValue := otherComposite.GetField(otherInterpreter, fieldName)

		fieldTrace := trace.Append(fmt.Sprintf(".%s", fieldName))
		fieldHasDifference := dr.diffValues(
			vInterpreter,
			fieldValue,
			otherInterpreter,
			otherFieldValue,
			domain,
			fieldTrace,
		)
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
	domain common.StorageDomain,
	trace *util.Trace,
) (hasDifference bool) {

	defer func() {
		if r := recover(); r != nil {
			dr.reportWriter.Write(
				diffProblem{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffErrorKindString[abortErrorKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"panic while diffing dictionary: %s",
						r,
					),
				},
			)
		}
	}()

	otherDictionary, ok := other.(*interpreter.DictionaryValue)
	if !ok {
		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueTypeDiffKind],
				Trace:   trace.String(),
				Msg:     fmt.Sprintf("types differ: %T != %T", v, other),
			})
		return true
	}

	if !v.Type.Equal(otherDictionary.Type) {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueStaticTypeDiffKind],
				Trace:   trace.String(),
				Msg: fmt.Sprintf(
					"dict static types differ: %s != %s",
					v.Type,
					otherDictionary.Type),
			})
	}

	oldKeys := make([]interpreter.Value, 0, v.Count())
	v.IterateKeys(vInterpreter, interpreter.EmptyLocationRange, func(key interpreter.Value) (resume bool) {
		oldKeys = append(oldKeys, key)
		return true
	})

	newKeys := make([]interpreter.Value, 0, otherDictionary.Count())
	otherDictionary.IterateKeys(otherInterpreter, interpreter.EmptyLocationRange, func(key interpreter.Value) (resume bool) {
		newKeys = append(newKeys, key)
		return true
	})

	onlyOldKeys := make([]interpreter.Value, 0, len(oldKeys))

	// Compare elements in both dict values

	for _, key := range oldKeys {
		valueTrace := trace.Append(fmt.Sprintf("[%v]", key))

		oldValue, _ := v.Get(vInterpreter, interpreter.EmptyLocationRange, key)

		newValue, found := otherDictionary.Get(otherInterpreter, interpreter.EmptyLocationRange, key)
		if !found {
			onlyOldKeys = append(onlyOldKeys, key)
			continue
		}

		elementHasDifference := dr.diffValues(
			vInterpreter,
			oldValue,
			otherInterpreter,
			newValue,
			domain,
			valueTrace,
		)
		if elementHasDifference {
			hasDifference = true
		}
	}

	// Log keys only present in old dict value

	if len(onlyOldKeys) > 0 {
		hasDifference = true

		dr.reportWriter.Write(
			difference{
				Address: dr.address.Hex(),
				Domain:  domain.Identifier(),
				Kind:    diffKindString[cadenceValueDiffKind],
				Trace:   trace.String(),
				Msg: fmt.Sprintf(
					"old dict value has %d elements with keys %v, that are not present in new dict value",
					len(onlyOldKeys),
					onlyOldKeys,
				),
			})
	}

	// Log keys only present in new dict value

	if len(oldKeys) != len(newKeys) || len(onlyOldKeys) > 0 {
		onlyNewKeys := make([]interpreter.Value, 0, len(newKeys))

		// find keys only present in new dict
		for _, key := range newKeys {
			found := v.ContainsKey(vInterpreter, interpreter.EmptyLocationRange, key)
			if !found {
				onlyNewKeys = append(onlyNewKeys, key)
			}
		}

		if len(onlyNewKeys) > 0 {
			hasDifference = true

			dr.reportWriter.Write(
				difference{
					Address: dr.address.Hex(),
					Domain:  domain.Identifier(),
					Kind:    diffKindString[cadenceValueDiffKind],
					Trace:   trace.String(),
					Msg: fmt.Sprintf(
						"new dict value has %d elements with keys %v, that are not present in old dict value",
						len(onlyNewKeys),
						onlyNewKeys,
					),
				})
		}
	}

	return hasDifference
}

func getStorageMapKeys(storageMap *interpreter.DomainStorageMap) []any {
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

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func newReadonlyStorage(regs registers.Registers) *runtime.Storage {
	ledger := &registers.ReadOnlyLedger{Registers: regs}
	config := runtime.StorageConfig{}
	return runtime.NewStorage(ledger, nil, config)
}

type readonlyStorageRuntime struct {
	Interpreter  *interpreter.Interpreter
	Storage      *runtime.Storage
	PayloadCount int
}

func newReadonlyStorageRuntimeWithStorage(storage *runtime.Storage, payloadCount int) (*readonlyStorageRuntime, error) {
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
		Interpreter:  inter,
		Storage:      storage,
		PayloadCount: payloadCount,
	}, nil
}
