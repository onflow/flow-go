package language

import (
	"fmt"
)

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
	ID() string
}

// revive:enable

// AnyType

type AnyType struct{}

func (AnyType) isType() {}

func (AnyType) ID() string {
	return "Any"
}

// AnyStructType

type AnyStructType struct{}

func (AnyStructType) isType() {}

func (AnyStructType) ID() string {
	return "AnyStruct"
}

// AnyResourceType

type AnyResourceType struct{}

func (AnyResourceType) isType() {}

func (AnyResourceType) ID() string {
	return "AnyResource"
}

// OptionalType

type OptionalType struct {
	Type Type
}

func (OptionalType) isType() {}

func (t OptionalType) ID() string {
	return fmt.Sprintf("%s?", t.Type)
}

// Variable

type Variable struct {
	Type Type
}

func (Variable) isType() {}

// TODO:
func (Variable) ID() string {
	panic("not implemented")
}

// VoidType

type VoidType struct{}

func (VoidType) isType() {}

func (VoidType) ID() string {
	return "Void"
}

// BoolType

type BoolType struct{}

func (BoolType) isType() {}

func (BoolType) ID() string {
	return "Bool"
}

// StringType

type StringType struct{}

func (StringType) isType() {}

func (StringType) ID() string {
	return "String"
}

// BytesType

type BytesType struct{}

func (BytesType) isType() {}

func (BytesType) ID() string {
	return "Bytes"
}

// AddressType

type AddressType struct{}

func (AddressType) isType() {}

func (AddressType) ID() string {
	return "Address"
}

// IntType

type IntType struct{}

func (IntType) isType() {}

func (IntType) ID() string {
	return "Int"
}

// Int8Type

type Int8Type struct{}

func (Int8Type) isType() {}

func (Int8Type) ID() string {
	return "Int8"
}

// Int16Type

type Int16Type struct{}

func (Int16Type) isType() {}

func (Int16Type) ID() string {
	return "Int16"
}

// Int32Type

type Int32Type struct{}

func (Int32Type) isType() {}

func (Int32Type) ID() string {
	return "Int32"
}

// Int64Type

type Int64Type struct{}

func (Int64Type) isType() {}

func (Int64Type) ID() string {
	return "Int64"
}

// UInt8Type

type UInt8Type struct{}

func (UInt8Type) isType() {}

func (UInt8Type) ID() string {
	return "UInt8"
}

// UInt16Type

type UInt16Type struct{}

func (UInt16Type) isType() {}

func (UInt16Type) ID() string {
	return "UInt16"
}

// UInt32Type

type UInt32Type struct{}

func (UInt32Type) isType() {}

func (UInt32Type) ID() string {
	return "UInt32"
}

// UInt64Type

type UInt64Type struct{}

func (UInt64Type) isType() {}

func (UInt64Type) ID() string {
	return "UInt64"
}

// VariableSizedArrayType

type VariableSizedArrayType struct {
	ElementType Type
}

func (VariableSizedArrayType) isType() {}

func (t VariableSizedArrayType) ID() string {
	return fmt.Sprintf("[%s]", t.ElementType.ID())
}

// ConstantSizedArrayType

type ConstantSizedArrayType struct {
	Size        uint
	ElementType Type
}

func (ConstantSizedArrayType) isType() {}

func (t ConstantSizedArrayType) ID() string {
	return fmt.Sprintf("[%s;%d]", t.ElementType.ID(), t.Size)
}

// DictionaryType

type DictionaryType struct {
	KeyType     Type
	ElementType Type
}

func (DictionaryType) isType() {}

func (t DictionaryType) ID() string {
	return fmt.Sprintf(
		"{%s:%s}",
		t.KeyType.ID(),
		t.ElementType.ID(),
	)
}

// Field

type Field struct {
	Identifier string
	Type       Type
}

// Parameter

type Parameter struct {
	Label      string
	Identifier string
	Type       Type
}

// CompositeType

type CompositeType struct {
	typeID       string
	Identifier   string
	Fields       []Field
	Initializers [][]Parameter
}

func (CompositeType) isType() {}

func (t CompositeType) ID() string {
	return t.typeID
}

func (t CompositeType) WithID(id string) CompositeType {
	t.typeID = id
	return t
}

// StructType

type StructType struct {
	CompositeType
}

// ResourceType

type ResourceType struct {
	CompositeType
}

// EventType

type EventType struct {
	CompositeType
}

// Function

type Function struct {
	typeID     string
	Identifier string
	Parameters []Parameter
	ReturnType Type
}

func (t Function) isType() {}

func (t Function) ID() string {
	return t.typeID
}

func (t Function) WithID(id string) Function {
	t.typeID = id
	return t
}

// FunctionType

type FunctionType struct {
	ParameterTypes []Type
	ReturnType     Type
}

func (FunctionType) isType() {}

// TODO:
func (FunctionType) ID() string {
	panic("not implemented")
}

// ResourcePointer

type ResourcePointer struct {
	TypeName string
}

func (ResourcePointer) isType() {}

func (t ResourcePointer) ID() string {
	return t.TypeName
}

// StructPointer

type StructPointer struct {
	TypeName string
}

func (StructPointer) isType() {}

func (t StructPointer) ID() string {
	return t.TypeName
}

// EventPointer

type EventPointer struct {
	TypeName string
}

func (EventPointer) isType() {}

func (t EventPointer) ID() string {
	return t.TypeName
}
