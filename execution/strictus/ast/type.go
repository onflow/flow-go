package ast

type Type interface {
	isType()
}

type Int32Type struct{}

func (Int32Type) isType() {}

type Int64Type struct{}

func (Int64Type) isType() {}

type DynamicType struct {
	Type
}

type FixedType struct {
	Type
	Size int
}
