package interpreter

import (
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

type DynamicType interface {
	IsDynamicType()
}

type ReferenceType interface {
	DynamicType
	isReferenceType()
	Authorized() bool
	InnerType() DynamicType
}

// VoidType

type VoidType struct{}

func (VoidType) IsDynamicType() {}

// StringType

type StringType struct{}

func (StringType) IsDynamicType() {}

// BoolType

type BoolType struct{}

func (BoolType) IsDynamicType() {}

// ArrayType

type ArrayType struct {
	ElementTypes []DynamicType
}

func (ArrayType) IsDynamicType() {}

// NumberType

type NumberType struct {
	StaticType sema.Type
}

func (NumberType) IsDynamicType() {}

// CompositeType

type CompositeType struct {
	StaticType sema.Type
}

func (CompositeType) IsDynamicType() {}

// DictionaryType

type DictionaryType struct {
	EntryTypes []struct{ KeyType, ValueType DynamicType }
}

func (DictionaryType) IsDynamicType() {}

// NilType

type NilType struct{}

func (NilType) IsDynamicType() {}

// SomeType

type SomeType struct {
	InnerType DynamicType
}

func (SomeType) IsDynamicType() {}

// StorageType

type StorageType struct{}

func (StorageType) IsDynamicType() {}

// StorageReferenceType

type StorageReferenceType struct {
	authorized bool
	innerType  DynamicType
}

func (StorageReferenceType) IsDynamicType() {}

func (StorageReferenceType) isReferenceType() {}

func (t StorageReferenceType) Authorized() bool {
	return t.authorized
}

func (t StorageReferenceType) InnerType() DynamicType {
	return t.innerType
}

// EphemeralReferenceType

type EphemeralReferenceType struct {
	authorized bool
	innerType  DynamicType
}

func (EphemeralReferenceType) IsDynamicType() {}

func (EphemeralReferenceType) isReferenceType() {}

func (t EphemeralReferenceType) Authorized() bool {
	return t.authorized
}

func (t EphemeralReferenceType) InnerType() DynamicType {
	return t.innerType
}

// AddressType

type AddressType struct{}

func (AddressType) IsDynamicType() {}

// PublishedType

type PublishedType struct{}

func (PublishedType) IsDynamicType() {}

// FunctionType

type FunctionType struct{}

func (FunctionType) IsDynamicType() {}
