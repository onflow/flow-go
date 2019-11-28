package types

import "fmt"

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
	ID() string
}

// revive:enable

type isAType struct{}

func (isAType) isType() {}

type Annotation struct {
	IsMove bool
	Type   Type
}

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

type Uint8 struct{ isAType }

func (Uint8) ID() string { return "Uint8" }

type Uint16 struct{ isAType }

func (Uint16) ID() string { return "Uint16" }

type Uint32 struct{ isAType }

func (Uint32) ID() string { return "Uint32" }

type Uint64 struct{ isAType }

func (Uint64) ID() string { return "Uint64" }

type VariableSizedArray struct {
	isAType
	ElementType Type
}

func (t VariableSizedArray) ID() string {
	return fmt.Sprintf("[%s]", t.ElementType.ID())
}

type ConstantSizedArray struct {
	isAType
	Size        int
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

type Composite struct {
	isAType
	TypeID     string
	FieldTypes []Type
}

func (t Composite) ID() string {
	return t.TypeID
}

type Event struct {
	isAType
	TypeID     string
	FieldTypes []EventField
}

func (t Event) ID() string {
	return t.TypeID
}

type EventField struct {
	isAType
	Identifier string
	Type       Type
}

type Function struct {
	isAType
	TypeID                   string
	ParameterTypeAnnotations []Annotation
	ReturnTypeAnnotation     Annotation
}

func (t Function) ID() string {
	return t.TypeID
}
