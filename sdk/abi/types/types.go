package types

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
}

// revive:enable

type isAType struct{}

func (isAType) isType() {}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{ isAType }

type Bool struct{ isAType }

type String struct{ isAType }

type Bytes struct{ isAType }

type Int struct{ isAType }

type Int8 struct{ isAType }

type Int16 struct{ isAType }

type Int32 struct{ isAType }

type Int64 struct{ isAType }

type Uint8 struct{ isAType }

type Uint16 struct{ isAType }

type Uint32 struct{ isAType }

type Uint64 struct{ isAType }

type VariableSizedArray struct {
	isAType
	ElementType Type
}

type ConstantSizedArray struct {
	isAType
	Size        int
	ElementType Type
}

type Composite struct {
	isAType
	FieldTypes []Type
}

type Dictionary struct {
	isAType
	KeyType     Type
	ElementType Type
}

type Function struct {
	isAType
	ParameterTypeAnnotations []Annotation
	ReturnTypeAnnotation     Annotation
}

type Event struct {
	isAType
	Identifier string
	FieldTypes []EventField
}

type EventField struct {
	isAType
	Identifier string
	Type       Type
}

type Address struct{ isAType }
