package types

import (
	"fmt"
)

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
	ID() string
}

// revive:enable

type isAType struct{}

func (isAType) isType() {}

type Any struct{ isAType }

func (Any) ID() string { return "Any" }

type Optional struct {
	isAType
	Type Type
}

func (t Optional) ID() string { return fmt.Sprintf("%s?", t.Type) }

type Variable struct {
	isAType
	Type Type
}

// TODO:
func (Variable) ID() string { return "NOT IMPLEMENTED" }

type Void struct{ isAType }

func (Void) ID() string { return "Void" }

type Bool struct{ isAType }

func (Bool) ID() string { return "Bool" }

type String struct{ isAType }

func (String) ID() string { return "String" }

type Bytes struct{ isAType }

func (Bytes) ID() string { return "Bytes" }

type Address struct{ isAType }

func (Address) ID() string { return "Address" }

type Int struct{ isAType }

func (Int) ID() string { return "Int" }

type Int8 struct{ isAType }

func (Int8) ID() string { return "Int8" }

type Int16 struct{ isAType }

func (Int16) ID() string { return "Int16" }

type Int32 struct{ isAType }

func (Int32) ID() string { return "Int32" }

type Int64 struct{ isAType }

func (Int64) ID() string { return "Int64" }

type UInt8 struct{ isAType }

func (UInt8) ID() string { return "UInt8" }

type UInt16 struct{ isAType }

func (UInt16) ID() string { return "UInt16" }

type UInt32 struct{ isAType }

func (UInt32) ID() string { return "UInt32" }

type UInt64 struct{ isAType }

func (UInt64) ID() string { return "UInt64" }

type VariableSizedArray struct {
	isAType
	ElementType Type
}

func (t VariableSizedArray) ID() string {
	return fmt.Sprintf("[%s]", t.ElementType.ID())
}

type ConstantSizedArray struct {
	isAType
	Size        uint
	ElementType Type
}

func (t ConstantSizedArray) ID() string {
	return fmt.Sprintf("[%s;%d]", t.ElementType.ID(), t.Size)
}

type Dictionary struct {
	isAType
	KeyType     Type
	ElementType Type
}

func (t Dictionary) ID() string {
	return fmt.Sprintf(
		"{%s:%s}",
		t.KeyType.ID(),
		t.ElementType.ID(),
	)
}

type Parameter struct {
	Label      string
	Identifier string
	Type       Type
}

type Composite struct {
	isAType
	TypeID       string
	Identifier   string
	Fields       map[string]Type
	Initializers [][]Parameter
}

func (t Composite) ID() string {
	return t.TypeID
}

type Struct struct {
	isAType
	Composite
}

type Resource struct {
	isAType
	Composite
}

type Function struct {
	isAType
	TypeID     string
	Identifier string
	Parameters []Parameter
	ReturnType Type
}

func (t Function) ID() string { return t.TypeID }

// A type representing anonymous function (aka without named arguments)
type FunctionType struct {
	isAType
	ParameterTypes []Type
	ReturnType     Type
}

// TODO:
func (t FunctionType) ID() string { return "NOT IMPLEMENTED" }

type Event struct {
	isAType
	TypeID      string
	Identifier  string
	Fields      map[string]Type
	Initializer []Parameter
}

func (t Event) ID() string {
	return t.TypeID
}

// Pointers are simply pointers to already existing types, to prevent circular references
type ResourcePointer struct {
	isAType
	TypeName string
}

func (t ResourcePointer) ID() string {
	return t.TypeName
}

type StructPointer struct {
	isAType
	TypeName string
}

func (t StructPointer) ID() string {
	return t.TypeName
}
