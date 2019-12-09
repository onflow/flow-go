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

type AnyStruct struct{ isAType }

type Int struct{ isAType }

type Int8 struct{ isAType }

type Int16 struct{ isAType }

type Int32 struct{ isAType }

type Int64 struct{ isAType }

type UInt8 struct{ isAType }

type UInt16 struct{ isAType }

type UInt32 struct{ isAType }

type UInt64 struct{ isAType }

type Variable struct {
	isAType
	Type Type
}

type VariableSizedArray struct {
	isAType
	ElementType Type
}

type ConstantSizedArray struct {
	isAType
	Size        uint
	ElementType Type
}

type Parameter struct {
	Field
	Label string
}

type Composite struct {
	isAType
	Fields       map[string]*Field
	Identifier   string
	Initializers [][]*Parameter
}

type Struct struct {
	isAType
	Composite
}

type Resource struct {
	isAType
	Composite
}

type Dictionary struct {
	isAType
	KeyType     Type
	ElementType Type
}

type Function struct {
	isAType
	Parameters []*Parameter
	ReturnType Type
}

// A type representing anonymous function (aka without named arguments)
type FunctionType struct {
	isAType
	ParameterTypes []Type
	ReturnType     Type
}

type Event struct {
	isAType
	Fields     []*Parameter
	Identifier string
}

type Field struct {
	isAType
	Identifier string
	Type       Type
}

type Optional struct {
	isAType
	Of Type
}

//Pointers are simply pointers to already existing types, to prevent circular references
type ResourcePointer struct {
	isAType
	TypeName string
}

type StructPointer struct {
	isAType
	TypeName string
}

type Address struct{ isAType }
