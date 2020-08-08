/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package runtime

import (
	"fmt"
	"sort"

	"example.com/cadence-initial"
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/interpreter"
	"example.com/cadence-initial/runtime/sema"
)

// exportValue converts a runtime value to its native Go representation.
func exportValue(value exportableValue) cadence.Value {
	return exportValueWithInterpreter(value.Value, value.Interpreter())
}

// exportEvent converts a runtime event to its native Go representation.
func exportEvent(event exportableEvent) cadence.Event {
	fields := make([]cadence.Value, len(event.Fields))

	for i, field := range event.Fields {
		fields[i] = exportValueWithInterpreter(field.Value, field.Interpreter())
	}

	return cadence.NewEvent(fields).WithType(exportType(event.Type).(cadence.EventType))
}

func exportValueWithInterpreter(value interpreter.Value, inter *interpreter.Interpreter) cadence.Value {
	switch v := value.(type) {
	case interpreter.VoidValue:
		return cadence.NewVoid()
	case interpreter.NilValue:
		return cadence.NewOptional(nil)
	case *interpreter.SomeValue:
		return exportSomeValue(v, inter)
	case interpreter.BoolValue:
		return cadence.NewBool(bool(v))
	case *interpreter.StringValue:
		return cadence.NewString(v.Str)
	case *interpreter.ArrayValue:
		return exportArrayValue(v, inter)
	case interpreter.IntValue:
		return cadence.NewIntFromBig(v.ToBigInt())
	case interpreter.Int8Value:
		return cadence.NewInt8(int8(v))
	case interpreter.Int16Value:
		return cadence.NewInt16(int16(v))
	case interpreter.Int32Value:
		return cadence.NewInt32(int32(v))
	case interpreter.Int64Value:
		return cadence.NewInt64(int64(v))
	case interpreter.Int128Value:
		return cadence.NewInt128FromBig(v.ToBigInt())
	case interpreter.Int256Value:
		return cadence.NewInt256FromBig(v.ToBigInt())
	case interpreter.UIntValue:
		return cadence.NewUIntFromBig(v.ToBigInt())
	case interpreter.UInt8Value:
		return cadence.NewUInt8(uint8(v))
	case interpreter.UInt16Value:
		return cadence.NewUInt16(uint16(v))
	case interpreter.UInt32Value:
		return cadence.NewUInt32(uint32(v))
	case interpreter.UInt64Value:
		return cadence.NewUInt64(uint64(v))
	case interpreter.UInt128Value:
		return cadence.NewUInt128FromBig(v.ToBigInt())
	case interpreter.UInt256Value:
		return cadence.NewUInt256FromBig(v.ToBigInt())
	case interpreter.Word8Value:
		return cadence.NewWord8(uint8(v))
	case interpreter.Word16Value:
		return cadence.NewWord16(uint16(v))
	case interpreter.Word32Value:
		return cadence.NewWord32(uint32(v))
	case interpreter.Word64Value:
		return cadence.NewWord64(uint64(v))
	case interpreter.Fix64Value:
		return cadence.NewFix64(int64(v))
	case interpreter.UFix64Value:
		return cadence.NewUFix64(uint64(v))
	case *interpreter.CompositeValue:
		return exportCompositeValue(v, inter)
	case *interpreter.DictionaryValue:
		return exportDictionaryValue(v, inter)
	case interpreter.AddressValue:
		return cadence.NewAddress(v)
	}

	panic(fmt.Sprintf("cannot convert value of type %T", value))
}

func exportSomeValue(v *interpreter.SomeValue, inter *interpreter.Interpreter) cadence.Value {
	if v.Value == nil {
		return cadence.NewOptional(nil)
	}

	value := exportValueWithInterpreter(v.Value, inter)

	return cadence.NewOptional(value)
}

func exportArrayValue(v *interpreter.ArrayValue, inter *interpreter.Interpreter) cadence.Value {
	values := make([]cadence.Value, len(v.Values))

	for i, value := range v.Values {
		values[i] = exportValueWithInterpreter(value, inter)
	}

	return cadence.NewArray(values)
}

func exportCompositeValue(v *interpreter.CompositeValue, inter *interpreter.Interpreter) cadence.Value {
	fields := make([]cadence.Value, len(v.Fields))

	keys := make([]string, 0, len(v.Fields))
	for key := range v.Fields {
		keys = append(keys, key)
	}

	// sort keys in lexicographical order
	sort.Strings(keys)

	for i, key := range keys {
		field := v.Fields[key]
		fields[i] = exportValueWithInterpreter(field, inter)
	}

	dynamicType := v.DynamicType(inter).(interpreter.CompositeDynamicType)
	staticType := dynamicType.StaticType.(*sema.CompositeType)

	t := exportType(staticType)

	switch staticType.Kind {
	case common.CompositeKindStructure:
		return cadence.NewStruct(fields).WithType(t.(cadence.StructType))
	case common.CompositeKindResource:
		return cadence.NewResource(fields).WithType(t.(cadence.ResourceType))
	case common.CompositeKindEvent:
		return cadence.NewEvent(fields).WithType(t.(cadence.EventType))
	}

	panic(fmt.Errorf("invalid composite kind `%s`, must be Struct, Resource or Event", staticType.Kind))
}

func exportDictionaryValue(v *interpreter.DictionaryValue, inter *interpreter.Interpreter) cadence.Value {
	pairs := make([]cadence.KeyValuePair, v.Count())

	for i, keyValue := range v.Keys.Values {
		key := keyValue.(interpreter.HasKeyString).KeyString()
		value := v.Entries[key]

		convertedKey := exportValueWithInterpreter(keyValue, inter)
		convertedValue := exportValueWithInterpreter(value, inter)

		pairs[i] = cadence.KeyValuePair{
			Key:   convertedKey,
			Value: convertedValue,
		}
	}

	return cadence.NewDictionary(pairs)
}

// importValue converts a Cadence value to a runtime value.
func importValue(value cadence.Value) interpreter.Value {
	switch v := value.(type) {
	case cadence.Void:
		return interpreter.VoidValue{}
	case cadence.Optional:
		return importOptionalValue(v)
	case cadence.Bool:
		return interpreter.BoolValue(v)
	case cadence.String:
		return interpreter.NewStringValue(string(v))
	case cadence.Bytes:
		return interpreter.ByteSliceToByteArrayValue(v)
	case cadence.Address:
		return interpreter.NewAddressValueFromBytes(v.Bytes())
	case cadence.Int:
		return interpreter.NewIntValueFromBigInt(v.Big())
	case cadence.Int8:
		return interpreter.Int8Value(v)
	case cadence.Int16:
		return interpreter.Int16Value(v)
	case cadence.Int32:
		return interpreter.Int32Value(v)
	case cadence.Int64:
		return interpreter.Int64Value(v)
	case cadence.Int128:
		return interpreter.NewInt128ValueFromBigInt(v.Big())
	case cadence.Int256:
		return interpreter.NewInt256ValueFromBigInt(v.Big())
	case cadence.UInt:
		return interpreter.NewUIntValueFromBigInt(v.Big())
	case cadence.UInt8:
		return interpreter.UInt8Value(v)
	case cadence.UInt16:
		return interpreter.UInt16Value(v)
	case cadence.UInt32:
		return interpreter.UInt32Value(v)
	case cadence.UInt64:
		return interpreter.UInt64Value(v)
	case cadence.UInt128:
		return interpreter.NewUInt128ValueFromBigInt(v.Big())
	case cadence.UInt256:
		return interpreter.NewUInt256ValueFromBigInt(v.Big())
	case cadence.Word8:
		return interpreter.Word8Value(v)
	case cadence.Word16:
		return interpreter.Word16Value(v)
	case cadence.Word32:
		return interpreter.Word32Value(v)
	case cadence.Word64:
		return interpreter.Word64Value(v)
	case cadence.Fix64:
		return interpreter.Fix64Value(v)
	case cadence.UFix64:
		return interpreter.UFix64Value(v)
	case cadence.Array:
		return importArrayValue(v)
	case cadence.Dictionary:
		return importDictionaryValue(v)
	case cadence.Struct:
		return importCompositeValue(common.CompositeKindStructure, v.StructType.ID(), v.StructType.Fields, v.Fields)
	case cadence.Resource:
		return importCompositeValue(common.CompositeKindResource, v.ResourceType.ID(), v.ResourceType.Fields, v.Fields)
	case cadence.Event:
		return importCompositeValue(common.CompositeKindEvent, v.EventType.ID(), v.EventType.Fields, v.Fields)
	}

	panic(fmt.Sprintf("cannot convert value of type %T", value))
}

func importOptionalValue(v cadence.Optional) interpreter.Value {
	if v.Value == nil {
		return interpreter.NilValue{}
	}

	innerValue := importValue(v.Value)
	return interpreter.NewSomeValueOwningNonCopying(innerValue)
}

func importArrayValue(v cadence.Array) *interpreter.ArrayValue {
	values := make([]interpreter.Value, len(v.Values))

	for i, elem := range v.Values {
		values[i] = importValue(elem)
	}

	return interpreter.NewArrayValueUnownedNonCopying(values...)
}

func importDictionaryValue(v cadence.Dictionary) *interpreter.DictionaryValue {
	keysAndValues := make([]interpreter.Value, len(v.Pairs)*2)

	for i, pair := range v.Pairs {
		keysAndValues[i*2] = importValue(pair.Key)
		keysAndValues[i*2+1] = importValue(pair.Value)
	}

	return interpreter.NewDictionaryValueUnownedNonCopying(keysAndValues...)
}

func importCompositeValue(
	kind common.CompositeKind,
	typeID string,
	fieldTypes []cadence.Field,
	fieldValues []cadence.Value,
) *interpreter.CompositeValue {
	fields := make(map[string]interpreter.Value, len(fieldTypes))

	for i := 0; i < len(fieldTypes) && i < len(fieldValues); i++ {
		fieldType := fieldTypes[i]
		fieldValue := fieldValues[i]
		fields[fieldType.Identifier] = importValue(fieldValue)
	}

	return interpreter.NewCompositeValue(
		ast.LocationFromTypeID(typeID),
		sema.TypeID(typeID),
		kind,
		fields,
		nil,
	)
}
