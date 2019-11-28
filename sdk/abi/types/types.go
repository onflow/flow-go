package types

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
	ID() string
	setID(string)
}

// revive:enable

type baseType struct {
	id string
}

func (baseType) isType() {}

func (t baseType) ID() string { return t.id }
func (t baseType) setID(id string) {
	t.id = id
}

// WithID annotates a type with an ID.
func WithID(id string, t Type) Type {
	t.setID(id)
	return t
}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{ baseType }

type Bool struct{ baseType }

type String struct{ baseType }

type Bytes struct{ baseType }

type Int struct{ baseType }

type Int8 struct{ baseType }

type Int16 struct{ baseType }

type Int32 struct{ baseType }

type Int64 struct{ baseType }

type Uint8 struct{ baseType }

type Uint16 struct{ baseType }

type Uint32 struct{ baseType }

type Uint64 struct{ baseType }

type VariableSizedArray struct {
	baseType
	ElementType Type
}

type ConstantSizedArray struct {
	baseType
	Size        int
	ElementType Type
}

type Dictionary struct {
	baseType
	KeyType     Type
	ElementType Type
}

type Composite struct {
	baseType
	FieldTypes []Type
}

type Event struct {
	baseType
	Identifier string
	FieldTypes []EventField
}

type EventField struct {
	baseType
	Identifier string
	Type       Type
}

type Function struct {
	baseType
	ParameterTypeAnnotations []Annotation
	ReturnTypeAnnotation     Annotation
}

type Address struct{ baseType }
