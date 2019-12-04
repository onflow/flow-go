package types

// revive:disable:redefines-builtin-id

type Type interface {
	isType()
}

// revive:enable

type IsAType struct{}

func (IsAType) isType() {}

type Annotation struct {
	IsMove bool
	Type   Type
}

type Void struct{ IsAType }

type Bool struct{ IsAType }

type String struct{ IsAType }

type Bytes struct{ IsAType }

type Any struct{ IsAType }

type Int struct{ IsAType }

type Int8 struct{ IsAType }

type Int16 struct{ IsAType }

type Int32 struct{ IsAType }

type Int64 struct{ IsAType }

type UInt8 struct{ IsAType }

type UInt16 struct{ IsAType }

type UInt32 struct{ IsAType }

type UInt64 struct{ IsAType }

type Variable struct {
	IsAType
	Type Type
}

type VariableSizedArray struct {
	IsAType
	ElementType Type
}

type ConstantSizedArray struct {
	IsAType
	Size        uint
	ElementType Type
}

type Parameter struct {
	Field
	Label string
}

type Composite struct {
	IsAType
	Fields       map[string]*Field
	Identifier   string
	Initializers [][]*Parameter
}

type Struct struct {
	IsAType
	Composite
}

type Resource struct {
	IsAType
	Composite
}

type Dictionary struct {
	IsAType
	KeyType     Type
	ElementType Type
}

type Function struct {
	IsAType
	Parameters []*Parameter
	ReturnType Type
}

// A type representing anonymous function (aka without named arguments)
type FunctionType struct {
	IsAType
	ParameterTypes []Type
	ReturnType     Type
}

type Event struct {
	IsAType
	Fields     []*Parameter
	Identifier string
}

type Field struct {
	IsAType
	Identifier string
	Type       Type
}

type Optional struct {
	IsAType
	Of Type
}

//Pointers are simply pointers to already existing types, to prevent circular references
type ResourcePointer struct {
	IsAType
	TypeName string
}

type StructPointer struct {
	IsAType
	TypeName string
}

type Address struct{ IsAType }
