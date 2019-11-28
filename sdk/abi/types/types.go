package types

import "fmt"

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
	ID() string
}

// revive:enable

type baseType struct{}

func (baseType) isType() {}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{ baseType }

func (Void) ID() string { return "Void" }

type Bool struct{ baseType }

func (Bool) ID() string { return "Bool" }

type String struct{ baseType }

func (String) ID() string { return "String" }

type Bytes struct{ baseType }

func (Bytes) ID() string { return "Bytes" }

type Address struct{ baseType }

func (Address) ID() string { return "Address" }

type Int struct{ baseType }

func (Int) ID() string { return "Int" }

type Int8 struct{ baseType }

func (Int8) ID() string { return "Int8" }

type Int16 struct{ baseType }

func (Int16) ID() string { return "Int16" }

type Int32 struct{ baseType }

func (Int32) ID() string { return "Int32" }

type Int64 struct{ baseType }

func (Int64) ID() string { return "Int64" }

type Uint8 struct{ baseType }

func (Uint8) ID() string { return "Uint8" }

type Uint16 struct{ baseType }

func (Uint16) ID() string { return "Uint16" }

type Uint32 struct{ baseType }

func (Uint32) ID() string { return "Uint32" }

type Uint64 struct{ baseType }

func (Uint64) ID() string { return "Uint64" }

type VariableSizedArray struct {
	baseType
	ElementType Type
}

func (t VariableSizedArray) ID() string {
	return fmt.Sprintf("[%s]", t.ElementType.ID())
}

type ConstantSizedArray struct {
	baseType
	Size        int
	ElementType Type
}

func (t ConstantSizedArray) ID() string {
	return fmt.Sprintf("[%s;%d]", t.ElementType.ID(), t.Size)
}

type Dictionary struct {
	baseType
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

type Composite struct {
	baseType
	TypeID     string
	FieldTypes []Type
}

func (t Composite) ID() string {
	return t.TypeID
}

type Event struct {
	baseType
	TypeID     string
	FieldTypes []EventField
}

func (t Event) ID() string {
	return t.TypeID
}

type EventField struct {
	baseType
	Identifier string
	Type       Type
}

type Function struct {
	baseType
	TypeID                   string
	ParameterTypeAnnotations []Annotation
	ReturnTypeAnnotation     Annotation
}

func (t Function) ID() string {
	return t.TypeID
}
