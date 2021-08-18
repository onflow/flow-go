package migrations

import (
	"fmt"

	newInter "github.com/onflow/cadence-latest/runtime/interpreter"
	oldInter "github.com/onflow/cadence-v0180/runtime/interpreter"
)

var _ oldInter.Visitor = &ValueConverter{}

type ValueConverter struct {
	result   newInter.Value
	newInter *newInter.Interpreter
	oldInter *oldInter.Interpreter
}

func NewValueConverter(oldInter *oldInter.Interpreter, newInter *newInter.Interpreter) *ValueConverter {
	return &ValueConverter{
		oldInter: oldInter,
		newInter: newInter,
	}
}

func (c *ValueConverter) Convert(value oldInter.Value) newInter.Value {
	prevResult := c.result
	c.result = nil

	defer func() {
		c.result = prevResult
	}()

	value.Accept(c.oldInter, c)

	return c.result
}

func (c *ValueConverter) VisitValue(_ *oldInter.Interpreter, value oldInter.Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitTypeValue(_ *oldInter.Interpreter, value oldInter.TypeValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitVoidValue(_ *oldInter.Interpreter, value oldInter.VoidValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitBoolValue(_ *oldInter.Interpreter, value oldInter.BoolValue) {
	c.result = newInter.BoolValue(value)
}

func (c *ValueConverter) VisitStringValue(_ *oldInter.Interpreter, value *oldInter.StringValue) {
	c.result = newInter.NewStringValue(value.Str)
}

func (c *ValueConverter) VisitArrayValue(_ *oldInter.Interpreter, value *oldInter.ArrayValue) bool {
	newElements := make([]newInter.Value, value.Count())

	for index, element := range value.Elements() {
		newElements[index] = c.Convert(element)
	}

	arrayStaticType := ConvertStaticType(value.StaticType()).(newInter.ArrayStaticType)

	c.result = newInter.NewArrayValue(arrayStaticType, c.newInter.Storage, newElements...)

	// Do not decent. We already visited children here.
	return false
}

func (c *ValueConverter) VisitIntValue(_ *oldInter.Interpreter, value oldInter.IntValue) {
	c.result = newInter.NewIntValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitInt8Value(_ *oldInter.Interpreter, value oldInter.Int8Value) {
	c.result = newInter.Int8Value(value)
}

func (c *ValueConverter) VisitInt16Value(_ *oldInter.Interpreter, value oldInter.Int16Value) {
	c.result = newInter.Int16Value(value)
}

func (c *ValueConverter) VisitInt32Value(_ *oldInter.Interpreter, value oldInter.Int32Value) {
	c.result = newInter.Int32Value(value)
}

func (c *ValueConverter) VisitInt64Value(_ *oldInter.Interpreter, value oldInter.Int64Value) {
	c.result = newInter.Int64Value(value)
}

func (c *ValueConverter) VisitInt128Value(_ *oldInter.Interpreter, value oldInter.Int128Value) {
	c.result = newInter.NewInt128ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitInt256Value(_ *oldInter.Interpreter, value oldInter.Int256Value) {
	c.result = newInter.NewInt256ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUIntValue(_ *oldInter.Interpreter, value oldInter.UIntValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitUInt8Value(_ *oldInter.Interpreter, value oldInter.UInt8Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitUInt16Value(_ *oldInter.Interpreter, value oldInter.UInt16Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitUInt32Value(_ *oldInter.Interpreter, value oldInter.UInt32Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitUInt64Value(_ *oldInter.Interpreter, value oldInter.UInt64Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitUInt128Value(_ *oldInter.Interpreter, value oldInter.UInt128Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitUInt256Value(_ *oldInter.Interpreter, value oldInter.UInt256Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitWord8Value(_ *oldInter.Interpreter, value oldInter.Word8Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitWord16Value(_ *oldInter.Interpreter, value oldInter.Word16Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitWord32Value(_ *oldInter.Interpreter, value oldInter.Word32Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitWord64Value(_ *oldInter.Interpreter, value oldInter.Word64Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitFix64Value(_ *oldInter.Interpreter, value oldInter.Fix64Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitUFix64Value(_ *oldInter.Interpreter, value oldInter.UFix64Value) {
	panic("implement me")
}

func (c *ValueConverter) VisitCompositeValue(_ *oldInter.Interpreter, value *oldInter.CompositeValue) bool {
	panic("implement me")
}

func (c *ValueConverter) VisitDictionaryValue(_ *oldInter.Interpreter, value *oldInter.DictionaryValue) bool {
	panic("implement me")
}

func (c *ValueConverter) VisitNilValue(_ *oldInter.Interpreter, value oldInter.NilValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitSomeValue(_ *oldInter.Interpreter, value *oldInter.SomeValue) bool {
	panic("implement me")
}

func (c *ValueConverter) VisitStorageReferenceValue(_ *oldInter.Interpreter, value *oldInter.StorageReferenceValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitEphemeralReferenceValue(_ *oldInter.Interpreter, value *oldInter.EphemeralReferenceValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitAddressValue(_ *oldInter.Interpreter, value oldInter.AddressValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitPathValue(_ *oldInter.Interpreter, value oldInter.PathValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitCapabilityValue(_ *oldInter.Interpreter, value oldInter.CapabilityValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitLinkValue(_ *oldInter.Interpreter, value oldInter.LinkValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitInterpretedFunctionValue(_ *oldInter.Interpreter, value oldInter.InterpretedFunctionValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitHostFunctionValue(_ *oldInter.Interpreter, value oldInter.HostFunctionValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitBoundFunctionValue(_ *oldInter.Interpreter, value oldInter.BoundFunctionValue) {
	panic("implement me")
}

func (c *ValueConverter) VisitDeployedContractValue(_ *oldInter.Interpreter, value oldInter.DeployedContractValue) {
	panic("implement me")
}

func ConvertStaticType(staticType oldInter.StaticType) newInter.StaticType {
	switch typ := staticType.(type) {
	case oldInter.CompositeStaticType:

	case oldInter.InterfaceStaticType:

	case oldInter.VariableSizedStaticType:
		return newInter.VariableSizedStaticType{
			Type: ConvertStaticType(typ.Type),
		}

	case oldInter.ConstantSizedStaticType:
		return newInter.ConstantSizedStaticType{
			Type: ConvertStaticType(typ.Type),
			Size: typ.Size,
		}

	case oldInter.DictionaryStaticType:

	case oldInter.OptionalStaticType:

	case *oldInter.RestrictedStaticType:

	case oldInter.ReferenceStaticType:

	case oldInter.CapabilityStaticType:

	case oldInter.PrimitiveStaticType:
		return ConvertPrimitiveStaticType(typ)
	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType))
	}

	panic("not implemented yet")
}

func ConvertPrimitiveStaticType(staticType oldInter.PrimitiveStaticType) newInter.PrimitiveStaticType {
	switch staticType {
	case oldInter.PrimitiveStaticTypeVoid:
		return newInter.PrimitiveStaticTypeVoid

	case oldInter.PrimitiveStaticTypeAny:
		return newInter.PrimitiveStaticTypeAny

	case oldInter.PrimitiveStaticTypeNever:
		return newInter.PrimitiveStaticTypeNever

	case oldInter.PrimitiveStaticTypeAnyStruct:
		return newInter.PrimitiveStaticTypeAnyStruct

	case oldInter.PrimitiveStaticTypeAnyResource:
		return newInter.PrimitiveStaticTypeAnyResource

	case oldInter.PrimitiveStaticTypeBool:
		return newInter.PrimitiveStaticTypeBool

	case oldInter.PrimitiveStaticTypeAddress:
		return newInter.PrimitiveStaticTypeAddress

	case oldInter.PrimitiveStaticTypeString:
		return newInter.PrimitiveStaticTypeString

	case oldInter.PrimitiveStaticTypeCharacter:
		return newInter.PrimitiveStaticTypeCharacter

	case oldInter.PrimitiveStaticTypeMetaType:
		return newInter.PrimitiveStaticTypeMetaType

	case oldInter.PrimitiveStaticTypeBlock:
		return newInter.PrimitiveStaticTypeBlock

	// Number

	case oldInter.PrimitiveStaticTypeNumber:
		return newInter.PrimitiveStaticTypeNumber
	case oldInter.PrimitiveStaticTypeSignedNumber:
		return newInter.PrimitiveStaticTypeSignedNumber

	// Integer
	case oldInter.PrimitiveStaticTypeInteger:
		return newInter.PrimitiveStaticTypeInteger
	case oldInter.PrimitiveStaticTypeSignedInteger:
		return newInter.PrimitiveStaticTypeSignedInteger

	// FixedPoint
	case oldInter.PrimitiveStaticTypeFixedPoint:
		return newInter.PrimitiveStaticTypeFixedPoint
	case oldInter.PrimitiveStaticTypeSignedFixedPoint:
		return newInter.PrimitiveStaticTypeSignedFixedPoint

	// Int*
	case oldInter.PrimitiveStaticTypeInt:
		return newInter.PrimitiveStaticTypeInt
	case oldInter.PrimitiveStaticTypeInt8:
		return newInter.PrimitiveStaticTypeInt8
	case oldInter.PrimitiveStaticTypeInt16:
		return newInter.PrimitiveStaticTypeInt16
	case oldInter.PrimitiveStaticTypeInt32:
		return newInter.PrimitiveStaticTypeInt32
	case oldInter.PrimitiveStaticTypeInt64:
		return newInter.PrimitiveStaticTypeInt64
	case oldInter.PrimitiveStaticTypeInt128:
		return newInter.PrimitiveStaticTypeInt128
	case oldInter.PrimitiveStaticTypeInt256:
		return newInter.PrimitiveStaticTypeInt256

	// UInt*
	case oldInter.PrimitiveStaticTypeUInt:
		return newInter.PrimitiveStaticTypeUInt
	case oldInter.PrimitiveStaticTypeUInt8:
		return newInter.PrimitiveStaticTypeUInt8
	case oldInter.PrimitiveStaticTypeUInt16:
		return newInter.PrimitiveStaticTypeUInt16
	case oldInter.PrimitiveStaticTypeUInt32:
		return newInter.PrimitiveStaticTypeUInt32
	case oldInter.PrimitiveStaticTypeUInt64:
		return newInter.PrimitiveStaticTypeUInt64
	case oldInter.PrimitiveStaticTypeUInt128:
		return newInter.PrimitiveStaticTypeUInt128
	case oldInter.PrimitiveStaticTypeUInt256:
		return newInter.PrimitiveStaticTypeUInt256

	// Word *

	case oldInter.PrimitiveStaticTypeWord8:
		return newInter.PrimitiveStaticTypeWord8
	case oldInter.PrimitiveStaticTypeWord16:
		return newInter.PrimitiveStaticTypeWord16
	case oldInter.PrimitiveStaticTypeWord32:
		return newInter.PrimitiveStaticTypeWord32
	case oldInter.PrimitiveStaticTypeWord64:
		return newInter.PrimitiveStaticTypeWord64

	// Fix*
	case oldInter.PrimitiveStaticTypeFix64:
		return newInter.PrimitiveStaticTypeFix64

	// UFix*
	case oldInter.PrimitiveStaticTypeUFix64:
		return newInter.PrimitiveStaticTypeUFix64

	// Storage

	case oldInter.PrimitiveStaticTypePath:
		return newInter.PrimitiveStaticTypePath
	case oldInter.PrimitiveStaticTypeStoragePath:
		return newInter.PrimitiveStaticTypeStoragePath
	case oldInter.PrimitiveStaticTypeCapabilityPath:
		return newInter.PrimitiveStaticTypeCapabilityPath
	case oldInter.PrimitiveStaticTypePublicPath:
		return newInter.PrimitiveStaticTypePublicPath
	case oldInter.PrimitiveStaticTypePrivatePath:
		return newInter.PrimitiveStaticTypePrivatePath
	case oldInter.PrimitiveStaticTypeCapability:
		return newInter.PrimitiveStaticTypeCapability
	case oldInter.PrimitiveStaticTypeAuthAccount:
		return newInter.PrimitiveStaticTypeAuthAccount
	case oldInter.PrimitiveStaticTypePublicAccount:
		return newInter.PrimitiveStaticTypePublicAccount
	case oldInter.PrimitiveStaticTypeDeployedContract:
		return newInter.PrimitiveStaticTypeDeployedContract
	case oldInter.PrimitiveStaticTypeAuthAccountContracts:
		return newInter.PrimitiveStaticTypeAuthAccountContracts
	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType.String()))
	}
}
