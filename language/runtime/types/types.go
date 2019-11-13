package types

type Type interface{}

type String struct{}

type Composite struct {
	FieldTypes []Type
}

type Dictionary struct {
	KeyType     Type
	ElementType Type
}
