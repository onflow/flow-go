package types

type Type interface {
	isType()
}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{}

func (Void) isType() {}

type Bool struct{}

func (Bool) isType() {}

type String struct{}

func (String) isType() {}

type Int struct{}

func (Int) isType() {}

type Int8 struct{}

func (Int8) isType() {}

type Int16 struct{}

func (Int16) isType() {}

type Int32 struct{}

func (Int32) isType() {}

type Int64 struct{}

func (Int64) isType() {}

type Uint8 struct{}

func (Uint8) isType() {}

type Uint16 struct{}

func (Uint16) isType() {}

type Uint32 struct{}

func (Uint32) isType() {}

type Uint64 struct{}

func (Uint64) isType() {}

type VariableSizedArray struct {
	ElementType Type
}

func (VariableSizedArray) isType() {}

type ConstantSizedArray struct {
	ElementType Type
}

func (ConstantSizedArray) isType() {}

type Composite struct {
	FieldTypes []Type
}

func (Composite) isType() {}

type Dictionary struct {
	KeyType     Type
	ElementType Type
}

func (Dictionary) isType() {}

type Function struct {
	ParameterTypeAnnotations []Annotation
	ReturnTypeAnnotation     Annotation
}

func (Function) isType() {}

type Event struct {
	FieldTypes []EventField
}

func (Event) isType() {}

type EventField struct {
	Identifier string
	Type       Type
}

type Address struct{}

func (Address) isType() {}
