package types

type Type interface{}

type Void struct{}

type Bool bool

type String struct{}

type Int struct{}

type Int8 struct{}

type Int16 struct{}

type Int32 struct{}

type Int64 struct{}

type Uint8 struct{}

type Uint16 struct{}

type Uint32 struct{}

type Uint64 struct{}

type Array struct {
	ElementType Type
}

type Composite struct {
	FieldTypes []Type
}

type Dictionary struct {
	KeyType     Type
	ElementType Type
}
