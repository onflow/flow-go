package migrations

import (
	oldInter "github.com/onflow/cadence0170/runtime/interpreter"
	newInter "github.com/onflow/cadence0180/runtime/interpreter"
)

var _ oldInter.Visitor = &ValueConverter{}

type ValueConverter struct {
	result newInter.Value
}

func (v *ValueConverter) Result() newInter.Value {
	return v.result
}

func (v *ValueConverter) Convert(interpreter *oldInter.Interpreter, value oldInter.Value) newInter.Value {
	prevResult := v.result
	v.result = nil

	defer func() {
		v.result = prevResult
	}()

	value.Accept(interpreter, v)

	return v.result
}

func (v *ValueConverter) VisitValue(_ *oldInter.Interpreter, value oldInter.Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitTypeValue(_ *oldInter.Interpreter, value oldInter.TypeValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitVoidValue(_ *oldInter.Interpreter, value oldInter.VoidValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitBoolValue(_ *oldInter.Interpreter, value oldInter.BoolValue) {
	v.result = newInter.BoolValue(value)
}

func (v *ValueConverter) VisitStringValue(_ *oldInter.Interpreter, value *oldInter.StringValue) {
	v.result = newInter.NewStringValue(value.Str)
}

func (v *ValueConverter) VisitArrayValue(interpreter *oldInter.Interpreter, value *oldInter.ArrayValue) bool {
	newElements := make([]newInter.Value, value.Count())

	for index, element := range value.Elements() {
		newElements[index] = v.Convert(interpreter, element)
	}

	v.result = newInter.NewArrayValue(newElements)

	// Do not decent. We already visited children here.
	return false
}

func (v *ValueConverter) VisitIntValue(_ *oldInter.Interpreter, value oldInter.IntValue) {
	v.result = newInter.NewIntValueFromBigInt(value.BigInt)
}

func (v *ValueConverter) VisitInt8Value(_ *oldInter.Interpreter, value oldInter.Int8Value) {
	v.result = newInter.Int8Value(value)
}

func (v *ValueConverter) VisitInt16Value(_ *oldInter.Interpreter, value oldInter.Int16Value) {
	v.result = newInter.Int16Value(value)
}

func (v *ValueConverter) VisitInt32Value(_ *oldInter.Interpreter, value oldInter.Int32Value) {
	v.result = newInter.Int32Value(value)
}

func (v *ValueConverter) VisitInt64Value(_ *oldInter.Interpreter, value oldInter.Int64Value) {
	v.result = newInter.Int64Value(value)
}

func (v *ValueConverter) VisitInt128Value(_ *oldInter.Interpreter, value oldInter.Int128Value) {
	v.result = newInter.NewInt128ValueFromBigInt(value.BigInt)
}

func (v *ValueConverter) VisitInt256Value(_ *oldInter.Interpreter, value oldInter.Int256Value) {
	v.result = newInter.NewInt256ValueFromBigInt(value.BigInt)
}

func (v *ValueConverter) VisitUIntValue(_ *oldInter.Interpreter, value oldInter.UIntValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitUInt8Value(_ *oldInter.Interpreter, value oldInter.UInt8Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitUInt16Value(_ *oldInter.Interpreter, value oldInter.UInt16Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitUInt32Value(_ *oldInter.Interpreter, value oldInter.UInt32Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitUInt64Value(_ *oldInter.Interpreter, value oldInter.UInt64Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitUInt128Value(_ *oldInter.Interpreter, value oldInter.UInt128Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitUInt256Value(_ *oldInter.Interpreter, value oldInter.UInt256Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitWord8Value(_ *oldInter.Interpreter, value oldInter.Word8Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitWord16Value(_ *oldInter.Interpreter, value oldInter.Word16Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitWord32Value(_ *oldInter.Interpreter, value oldInter.Word32Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitWord64Value(_ *oldInter.Interpreter, value oldInter.Word64Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitFix64Value(_ *oldInter.Interpreter, value oldInter.Fix64Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitUFix64Value(_ *oldInter.Interpreter, value oldInter.UFix64Value) {
	panic("implement me")
}

func (v *ValueConverter) VisitCompositeValue(_ *oldInter.Interpreter, value *oldInter.CompositeValue) bool {
	panic("implement me")
}

func (v *ValueConverter) VisitDictionaryValue(_ *oldInter.Interpreter, value *oldInter.DictionaryValue) bool {
	panic("implement me")
}

func (v *ValueConverter) VisitNilValue(_ *oldInter.Interpreter, value oldInter.NilValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitSomeValue(_ *oldInter.Interpreter, value *oldInter.SomeValue) bool {
	panic("implement me")
}

func (v *ValueConverter) VisitStorageReferenceValue(_ *oldInter.Interpreter, value *oldInter.StorageReferenceValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitEphemeralReferenceValue(_ *oldInter.Interpreter, value *oldInter.EphemeralReferenceValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitAddressValue(_ *oldInter.Interpreter, value oldInter.AddressValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitPathValue(_ *oldInter.Interpreter, value oldInter.PathValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitCapabilityValue(_ *oldInter.Interpreter, value oldInter.CapabilityValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitLinkValue(_ *oldInter.Interpreter, value oldInter.LinkValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitInterpretedFunctionValue(_ *oldInter.Interpreter, value oldInter.InterpretedFunctionValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitHostFunctionValue(_ *oldInter.Interpreter, value oldInter.HostFunctionValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitBoundFunctionValue(_ *oldInter.Interpreter, value oldInter.BoundFunctionValue) {
	panic("implement me")
}

func (v *ValueConverter) VisitDeployedContractValue(_ *oldInter.Interpreter, value oldInter.DeployedContractValue) {
	panic("implement me")
}
