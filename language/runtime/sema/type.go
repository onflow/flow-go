package sema

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

type Type interface {
	isType()
	String() string
	Equal(other Type) bool
	IsResourceType() bool
	IsInvalidType() bool
}

type ExportableType interface {
	Export() types.Type
}

// ValueIndexableType

type ValueIndexableType interface {
	Type
	isValueIndexableType() bool
	ElementType(isAssignment bool) Type
	IndexingType() Type
}

// TypeIndexableType

type TypeIndexableType interface {
	Type
	isTypeIndexableType()
	ElementType(indexingType Type, isAssignment bool) Type
}

// TypeAnnotation

type TypeAnnotation struct {
	Move bool
	Type Type
}

func (a *TypeAnnotation) Export() types.Annotation {
	return types.Annotation{
		IsMove: a.Move,
		Type:   a.Type.(ExportableType).Export(),
	}
}

func (a *TypeAnnotation) String() string {
	if a.Move {
		return fmt.Sprintf("<-%s", a.Type)
	} else {
		return fmt.Sprint(a.Type)
	}
}

func (a *TypeAnnotation) Equal(other *TypeAnnotation) bool {
	return a.Move == other.Move &&
		a.Type.Equal(other.Type)
}

func NewTypeAnnotation(ty Type) *TypeAnnotation {
	return &TypeAnnotation{
		Move: ty.IsResourceType(),
		Type: ty,
	}
}

func NewTypeAnnotations(types ...Type) []*TypeAnnotation {
	typeAnnotations := make([]*TypeAnnotation, len(types))
	for i, ty := range types {
		typeAnnotations[i] = NewTypeAnnotation(ty)
	}
	return typeAnnotations
}

// AnyType represents the top type
type AnyType struct{}

func (*AnyType) isType() {}

func (*AnyType) String() string {
	return "Any"
}

func (*AnyType) Equal(other Type) bool {
	_, ok := other.(*AnyType)
	return ok
}

func (*AnyType) IsResourceType() bool {
	return false
}

func (*AnyType) IsInvalidType() bool {
	return false
}

// NeverType represents the bottom type
type NeverType struct{}

func (*NeverType) isType() {}

func (*NeverType) String() string {
	return "Never"
}

func (*NeverType) Equal(other Type) bool {
	_, ok := other.(*NeverType)
	return ok
}

func (*NeverType) IsResourceType() bool {
	return false
}

func (*NeverType) IsInvalidType() bool {
	return false
}

// VoidType represents the void type
type VoidType struct{}

func (*VoidType) isType() {}

func (*VoidType) Export() types.Type {
	return types.Void{}
}

func (*VoidType) String() string {
	return "Void"
}

func (*VoidType) Equal(other Type) bool {
	_, ok := other.(*VoidType)
	return ok
}

func (*VoidType) IsResourceType() bool {
	return false
}

func (*VoidType) IsInvalidType() bool {
	return false
}

// InvalidType represents a type that is invalid.
// It is the result of type checking failing and
// can't be expressed in programs.
//
type InvalidType struct{}

func (*InvalidType) isType() {}

func (*InvalidType) String() string {
	return "<<invalid>>"
}

func (*InvalidType) Equal(other Type) bool {
	_, ok := other.(*InvalidType)
	return ok
}

func (*InvalidType) IsResourceType() bool {
	return false
}

func (*InvalidType) IsInvalidType() bool {
	return true
}

// OptionalType represents the optional variant of another type
type OptionalType struct {
	Type Type
}

func (*OptionalType) isType() {}

func (t *OptionalType) String() string {
	if t.Type == nil {
		return "optional"
	}
	return fmt.Sprintf("%s?", t.Type)
}

func (t *OptionalType) Equal(other Type) bool {
	otherOptional, ok := other.(*OptionalType)
	if !ok {
		return false
	}
	return t.Type.Equal(otherOptional.Type)
}

func (t *OptionalType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

func (t *OptionalType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

// BoolType represents the boolean type
type BoolType struct{}

func (*BoolType) isType() {}

func (*BoolType) Export() types.Type {
	return types.Bool{}
}

func (*BoolType) String() string {
	return "Bool"
}

func (*BoolType) Equal(other Type) bool {
	_, ok := other.(*BoolType)
	return ok
}

func (*BoolType) IsResourceType() bool {
	return false
}

func (*BoolType) IsInvalidType() bool {
	return false
}

// CharacterType represents the character type

type CharacterType struct{}

func (*CharacterType) isType() {}

func (*CharacterType) String() string {
	return "Character"
}

func (*CharacterType) Equal(other Type) bool {
	_, ok := other.(*CharacterType)
	return ok
}

func (*CharacterType) IsResourceType() bool {
	return false
}

func (*CharacterType) IsInvalidType() bool {
	return false
}

// StringType represents the string type
type StringType struct{}

func (*StringType) isType() {}

func (*StringType) Export() types.Type {
	return types.String{}
}

func (*StringType) String() string {
	return "String"
}

func (*StringType) Equal(other Type) bool {
	_, ok := other.(*StringType)
	return ok
}

func (*StringType) IsResourceType() bool {
	return false
}

func (*StringType) IsInvalidType() bool {
	return false
}

func (*StringType) HasMembers() bool {
	return true
}

func (t *StringType) GetMember(field string, _ ast.Range, _ func(error)) *Member {
	switch field {
	case "length":
		return NewMemberForType(t, "length", Member{
			Type:         &IntType{},
			VariableKind: ast.VariableKindConstant,
		})
	case "concat":
		return NewMemberForType(t, "concat", Member{
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					&StringType{},
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&StringType{},
				),
			},
		})
	case "slice":
		return NewMemberForType(t, "slice", Member{
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					&IntType{},
					&IntType{},
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&StringType{},
				),
			},
			ArgumentLabels: []string{"from", "upTo"},
		})
	default:
		return nil
	}
}

func (t *StringType) isValueIndexableType() bool {
	return true
}

func (t *StringType) ElementType(isAssignment bool) Type {
	return &CharacterType{}
}

func (t *StringType) IndexingType() Type {
	return &IntegerType{}
}

// Ranged

type Ranged interface {
	Min() *big.Int
	Max() *big.Int
}

// IntegerType represents the super-type of all integer types
type IntegerType struct{}

func (*IntegerType) isType() {}

func (*IntegerType) String() string {
	return "integer"
}

func (*IntegerType) Equal(other Type) bool {
	_, ok := other.(*IntegerType)
	return ok
}

func (*IntegerType) IsResourceType() bool {
	return false
}

func (*IntegerType) IsInvalidType() bool {
	return false
}

func (*IntegerType) Min() *big.Int {
	return nil
}

func (*IntegerType) Max() *big.Int {
	return nil
}

// IntType represents the arbitrary-precision integer type `Int`
type IntType struct{}

func (*IntType) isType() {}

func (*IntType) Export() types.Type {
	return types.Int{}
}

func (*IntType) String() string {
	return "Int"
}

func (*IntType) Equal(other Type) bool {
	_, ok := other.(*IntType)
	return ok
}

func (*IntType) IsResourceType() bool {
	return false
}

func (*IntType) IsInvalidType() bool {
	return false
}

func (*IntType) Min() *big.Int {
	return nil
}

func (*IntType) Max() *big.Int {
	return nil
}

// Int8Type represents the 8-bit signed integer type `Int8`

type Int8Type struct{}

func (*Int8Type) isType() {}

func (*Int8Type) Export() types.Type {
	return types.Int8{}
}

func (*Int8Type) String() string {
	return "Int8"
}

func (*Int8Type) Equal(other Type) bool {
	_, ok := other.(*Int8Type)
	return ok
}

func (*Int8Type) IsResourceType() bool {
	return false
}

func (*Int8Type) IsInvalidType() bool {
	return false
}

var Int8TypeMin = big.NewInt(0).SetInt64(math.MinInt8)
var Int8TypeMax = big.NewInt(0).SetInt64(math.MaxInt8)

func (*Int8Type) Min() *big.Int {
	return Int8TypeMin
}

func (*Int8Type) Max() *big.Int {
	return Int8TypeMax
}

// Int16Type represents the 16-bit signed integer type `Int16`
type Int16Type struct{}

func (*Int16Type) isType() {}

func (*Int16Type) Export() types.Type {
	return types.Int16{}
}

func (*Int16Type) String() string {
	return "Int16"
}

func (*Int16Type) Equal(other Type) bool {
	_, ok := other.(*Int16Type)
	return ok
}

func (*Int16Type) IsResourceType() bool {
	return false
}

func (*Int16Type) IsInvalidType() bool {
	return false
}

var Int16TypeMin = big.NewInt(0).SetInt64(math.MinInt16)
var Int16TypeMax = big.NewInt(0).SetInt64(math.MaxInt16)

func (*Int16Type) Min() *big.Int {
	return Int16TypeMin
}

func (*Int16Type) Max() *big.Int {
	return Int16TypeMax
}

// Int32Type represents the 32-bit signed integer type `Int32`
type Int32Type struct{}

func (*Int32Type) isType() {}

func (*Int32Type) Export() types.Type {
	return types.Int32{}
}

func (*Int32Type) String() string {
	return "Int32"
}

func (*Int32Type) Equal(other Type) bool {
	_, ok := other.(*Int32Type)
	return ok
}

func (*Int32Type) IsResourceType() bool {
	return false
}

func (*Int32Type) IsInvalidType() bool {
	return false
}

var Int32TypeMin = big.NewInt(0).SetInt64(math.MinInt32)
var Int32TypeMax = big.NewInt(0).SetInt64(math.MaxInt32)

func (*Int32Type) Min() *big.Int {
	return Int32TypeMin
}

func (*Int32Type) Max() *big.Int {
	return Int32TypeMax
}

// Int64Type represents the 64-bit signed integer type `Int64`
type Int64Type struct{}

func (*Int64Type) isType() {}

func (*Int64Type) Export() types.Type {
	return types.Int64{}
}

func (*Int64Type) String() string {
	return "Int64"
}

func (*Int64Type) Equal(other Type) bool {
	_, ok := other.(*Int64Type)
	return ok
}

func (*Int64Type) IsResourceType() bool {
	return false
}

func (*Int64Type) IsInvalidType() bool {
	return false
}

var Int64TypeMin = big.NewInt(0).SetInt64(math.MinInt64)
var Int64TypeMax = big.NewInt(0).SetInt64(math.MaxInt64)

func (*Int64Type) Min() *big.Int {
	return Int64TypeMin
}

func (*Int64Type) Max() *big.Int {
	return Int64TypeMax
}

// UInt8Type represents the 8-bit unsigned integer type `UInt8`
type UInt8Type struct{}

func (*UInt8Type) isType() {}

func (*UInt8Type) Export() types.Type {
	return types.Uint8{}
}

func (*UInt8Type) String() string {
	return "UInt8"
}

func (*UInt8Type) Equal(other Type) bool {
	_, ok := other.(*UInt8Type)
	return ok
}

func (*UInt8Type) IsResourceType() bool {
	return false
}

func (*UInt8Type) IsInvalidType() bool {
	return false
}

var UInt8TypeMin = big.NewInt(0)
var UInt8TypeMax = big.NewInt(0).SetUint64(math.MaxUint8)

func (*UInt8Type) Min() *big.Int {
	return UInt8TypeMin
}

func (*UInt8Type) Max() *big.Int {
	return UInt8TypeMax
}

// UInt16Type represents the 16-bit unsigned integer type `UInt16`
type UInt16Type struct{}

func (*UInt16Type) isType() {}

func (*UInt16Type) Export() types.Type {
	return types.Uint16{}
}

func (*UInt16Type) String() string {
	return "UInt16"
}

func (*UInt16Type) Equal(other Type) bool {
	_, ok := other.(*UInt16Type)
	return ok
}

func (*UInt16Type) IsResourceType() bool {
	return false
}

func (*UInt16Type) IsInvalidType() bool {
	return false
}

var UInt16TypeMin = big.NewInt(0)
var UInt16TypeMax = big.NewInt(0).SetUint64(math.MaxUint16)

func (*UInt16Type) Min() *big.Int {
	return UInt16TypeMin
}

func (*UInt16Type) Max() *big.Int {
	return UInt16TypeMax
}

// UInt32Type represents the 32-bit unsigned integer type `UInt32`
type UInt32Type struct{}

func (*UInt32Type) isType() {}

func (*UInt32Type) Export() types.Type {
	return types.Uint32{}
}

func (*UInt32Type) String() string {
	return "UInt32"
}

func (*UInt32Type) Equal(other Type) bool {
	_, ok := other.(*UInt32Type)
	return ok
}

func (*UInt32Type) IsResourceType() bool {
	return false
}

func (*UInt32Type) IsInvalidType() bool {
	return false
}

var UInt32TypeMin = big.NewInt(0)
var UInt32TypeMax = big.NewInt(0).SetUint64(math.MaxUint32)

func (*UInt32Type) Min() *big.Int {
	return UInt32TypeMin
}

func (*UInt32Type) Max() *big.Int {
	return UInt32TypeMax
}

// UInt64Type represents the 64-bit unsigned integer type `UInt64`
type UInt64Type struct{}

func (*UInt64Type) isType() {}

func (*UInt64Type) Export() types.Type {
	return types.Uint64{}
}

func (*UInt64Type) String() string {
	return "UInt64"
}

func (*UInt64Type) Equal(other Type) bool {
	_, ok := other.(*UInt64Type)
	return ok
}

func (*UInt64Type) IsResourceType() bool {
	return false
}

func (*UInt64Type) IsInvalidType() bool {
	return false
}

var UInt64TypeMin = big.NewInt(0)
var UInt64TypeMax = big.NewInt(0).SetUint64(math.MaxUint64)

func (*UInt64Type) Min() *big.Int {
	return UInt64TypeMin
}

func (*UInt64Type) Max() *big.Int {
	return UInt64TypeMax
}

// ArrayType

type ArrayType interface {
	ValueIndexableType
	isArrayType()
}

func getArrayMember(t ArrayType, field string, targetRange ast.Range, report func(error)) *Member {

	switch field {
	case "append":
		// Appending elements to a constant sized array is not allowed

		if _, isConstantSized := t.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := t.ElementType(false)
		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					elementType,
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&VoidType{},
				),
			},
		})

	case "concat":
		// TODO: maybe allow constant sized:
		//    concatenate with variable sized and return variable sized

		if _, isConstantSized := t.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		// TODO: maybe allow for resource element type

		elementType := t.ElementType(false)

		if elementType.IsResourceType() {
			report(
				&InvalidResourceArrayMemberError{
					Name:  field,
					Range: targetRange,
				},
			)
		}

		typeAnnotation := NewTypeAnnotation(t)
		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: []*TypeAnnotation{
					typeAnnotation,
				},
				ReturnTypeAnnotation: typeAnnotation,
			},
		})

	case "insert":
		// Inserting elements into to a constant sized array is not allowed

		if _, isConstantSized := t.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := t.ElementType(false)
		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					&IntegerType{},
					elementType,
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&VoidType{},
				),
			},
			ArgumentLabels: []string{"at", ArgumentLabelNotRequired},
		})

	case "remove":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := t.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := t.ElementType(false)

		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					&IntegerType{},
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					elementType,
				),
			},
			ArgumentLabels: []string{"at"},
		})

	case "removeFirst":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := t.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := t.ElementType(false)

		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ReturnTypeAnnotation: NewTypeAnnotation(
					elementType,
				),
			},
		})

	case "removeLast":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := t.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := t.ElementType(false)

		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ReturnTypeAnnotation: NewTypeAnnotation(
					elementType,
				),
			},
		})

	case "contains":
		elementType := t.ElementType(false)

		// It impossible for an array of resources to have a `contains` function:
		// if the resource is passed as an argument, it cannot be inside the array

		if elementType.IsResourceType() {
			report(
				&InvalidResourceArrayMemberError{
					Name:  field,
					Range: targetRange,
				},
			)
		}

		// TODO: implement Equatable interface: https://github.com/dapperlabs/bamboo-node/issues/78

		if !IsEquatableType(elementType) {
			report(
				&NotEquatableTypeError{
					Type:  elementType,
					Range: targetRange,
				},
			)
		}

		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					elementType,
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&BoolType{},
				),
			},
		})

	case "length":
		return NewMemberForType(t, field, Member{
			Type:         &IntType{},
			VariableKind: ast.VariableKindConstant,
		})

	default:
		return nil
	}
}

// VariableSizedType is a variable sized array type
type VariableSizedType struct {
	Type
}

func (*VariableSizedType) isType()      {}
func (*VariableSizedType) isArrayType() {}

func (t *VariableSizedType) Export() types.Type {
	return types.VariableSizedArray{
		ElementType: t.Type.(ExportableType).Export(),
	}
}

func (t *VariableSizedType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
}

func (t *VariableSizedType) Equal(other Type) bool {
	otherArray, ok := other.(*VariableSizedType)
	if !ok {
		return false
	}

	return t.Type.Equal(otherArray.Type)
}

func (t *VariableSizedType) HasMembers() bool {
	return true
}

func (t *VariableSizedType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	return getArrayMember(t, identifier, targetRange, report)
}

func (t *VariableSizedType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

func (t *VariableSizedType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *VariableSizedType) isValueIndexableType() bool {
	return true
}

func (t *VariableSizedType) ElementType(isAssignment bool) Type {
	return t.Type
}

func (t *VariableSizedType) IndexingType() Type {
	return &IntegerType{}
}

// ConstantSizedType is a constant sized array type
type ConstantSizedType struct {
	Type
	Size int
}

func (*ConstantSizedType) isType()      {}
func (*ConstantSizedType) isArrayType() {}

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
}

func (t *ConstantSizedType) Equal(other Type) bool {
	otherArray, ok := other.(*ConstantSizedType)
	if !ok {
		return false
	}

	return t.Type.Equal(otherArray.Type) &&
		t.Size == otherArray.Size
}

func (t *ConstantSizedType) HasMembers() bool {
	return true
}

func (t *ConstantSizedType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	return getArrayMember(t, identifier, targetRange, report)
}

func (t *ConstantSizedType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

func (t *ConstantSizedType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *ConstantSizedType) isValueIndexableType() bool {
	return true
}

func (t *ConstantSizedType) ElementType(isAssignment bool) Type {
	return t.Type
}

func (t *ConstantSizedType) IndexingType() Type {
	return &IntegerType{}
}

// InvokableType

type InvokableType interface {
	Type
	InvocationFunctionType() *FunctionType
	CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression)
}

// FunctionType

type FunctionType struct {
	ParameterTypeAnnotations []*TypeAnnotation
	ReturnTypeAnnotation     *TypeAnnotation
	GetReturnType            func(argumentTypes []Type) Type
	RequiredArgumentCount    *int
}

func (*FunctionType) isType() {}

func (t *FunctionType) Export() types.Type {
	parameterTypeAnnotations := make([]types.Annotation, len(t.ParameterTypeAnnotations))

	for i, annotation := range t.ParameterTypeAnnotations {
		parameterTypeAnnotations[i] = annotation.Export()
	}

	return types.Function{
		ParameterTypeAnnotations: parameterTypeAnnotations,
		ReturnTypeAnnotation:     t.ReturnTypeAnnotation.Export(),
	}
}

func (t *FunctionType) InvocationFunctionType() *FunctionType {
	return t
}

func (*FunctionType) CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression) {
	// NO-OP: no checks for normal functions
}

func (t *FunctionType) String() string {
	var parameters strings.Builder
	for i, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		if i > 0 {
			parameters.WriteString(", ")
		}
		parameters.WriteString(parameterTypeAnnotation.String())
	}

	return fmt.Sprintf(
		"((%s): %s)",
		parameters.String(),
		t.ReturnTypeAnnotation,
	)
}

func (t *FunctionType) Equal(other Type) bool {
	otherFunction, ok := other.(*FunctionType)
	if !ok {
		return false
	}

	if len(t.ParameterTypeAnnotations) != len(otherFunction.ParameterTypeAnnotations) {
		return false
	}

	for i, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		otherParameterType := otherFunction.ParameterTypeAnnotations[i]
		if !parameterTypeAnnotation.Equal(otherParameterType) {
			return false
		}
	}

	return t.ReturnTypeAnnotation.Equal(otherFunction.ReturnTypeAnnotation)
}

func (*FunctionType) IsResourceType() bool {
	return false
}

func (t *FunctionType) IsInvalidType() bool {
	if t.ReturnTypeAnnotation.Type.IsInvalidType() {
		return true
	}

	for _, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		if parameterTypeAnnotation.Type.IsInvalidType() {
			return true
		}
	}

	return false
}

// SpecialFunctionType is the the type representing a special function,
// i.e., a constructor or destructor

type SpecialFunctionType struct {
	*FunctionType
}

// CheckedFunctionType is the the type representing a function that checks the arguments,
// e.g., integer functions

type CheckedFunctionType struct {
	*FunctionType
	ArgumentExpressionsCheck func(checker *Checker, argumentExpressions []ast.Expression)
}

func (t *CheckedFunctionType) CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression) {
	t.ArgumentExpressionsCheck(checker, argumentExpressions)
}

// baseTypes are the nominal types available in programs

var baseTypes map[string]Type

func init() {

	baseTypes = map[string]Type{
		"": &VoidType{},
	}

	types := []Type{
		&VoidType{},
		&AnyType{},
		&NeverType{},
		&BoolType{},
		&CharacterType{},
		&IntType{},
		&StringType{},
		&Int8Type{},
		&Int16Type{},
		&Int32Type{},
		&Int64Type{},
		&UInt8Type{},
		&UInt16Type{},
		&UInt32Type{},
		&UInt64Type{},
		&AddressType{},
	}

	for _, ty := range types {
		typeName := ty.String()

		// check type is not accidentally redeclared
		if _, ok := baseTypes[typeName]; ok {
			panic(errors.NewUnreachableError())
		}

		baseTypes[typeName] = ty
	}
}

// baseValues are the values available in programs

var BaseValues map[string]ValueDeclaration

type baseFunction struct {
	name           string
	invokableType  InvokableType
	argumentLabels []string
}

func (f baseFunction) ValueDeclarationType() Type {
	return f.invokableType
}

func (baseFunction) ValueDeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (baseFunction) ValueDeclarationPosition() ast.Position {
	return ast.Position{}
}

func (baseFunction) ValueDeclarationIsConstant() bool {
	return true
}

func (f baseFunction) ValueDeclarationArgumentLabels() []string {
	return f.argumentLabels
}

func init() {
	BaseValues = map[string]ValueDeclaration{}
	initIntegerFunctions()
	initAddressFunction()
}

func initIntegerFunctions() {
	integerTypes := []Type{
		&IntType{},
		&Int8Type{},
		&Int16Type{},
		&Int32Type{},
		&Int64Type{},
		&UInt8Type{},
		&UInt16Type{},
		&UInt32Type{},
		&UInt64Type{},
	}

	for _, integerType := range integerTypes {
		typeName := integerType.String()

		// check type is not accidentally redeclared
		if _, ok := BaseValues[typeName]; ok {
			panic(errors.NewUnreachableError())
		}

		BaseValues[typeName] = baseFunction{
			name: typeName,
			invokableType: &CheckedFunctionType{
				FunctionType: &FunctionType{
					ParameterTypeAnnotations: []*TypeAnnotation{{Type: &IntegerType{}}},
					ReturnTypeAnnotation:     &TypeAnnotation{Type: integerType},
				},
				ArgumentExpressionsCheck: integerFunctionArgumentExpressionsChecker(integerType),
			},
		}
	}
}

func initAddressFunction() {
	addressType := &AddressType{}
	typeName := addressType.String()

	// check type is not accidentally redeclared
	if _, ok := BaseValues[typeName]; ok {
		panic(errors.NewUnreachableError())
	}

	BaseValues[typeName] = baseFunction{
		name: typeName,
		invokableType: &CheckedFunctionType{
			FunctionType: &FunctionType{
				ParameterTypeAnnotations: []*TypeAnnotation{{Type: &IntegerType{}}},
				ReturnTypeAnnotation:     &TypeAnnotation{Type: addressType},
			},
			ArgumentExpressionsCheck: func(checker *Checker, argumentExpressions []ast.Expression) {
				intExpression, ok := argumentExpressions[0].(*ast.IntExpression)
				if !ok {
					return
				}
				checker.checkAddressLiteral(intExpression)
			},
		},
	}
}

func integerFunctionArgumentExpressionsChecker(integerType Type) func(*Checker, []ast.Expression) {
	return func(checker *Checker, argumentExpressions []ast.Expression) {
		intExpression, ok := argumentExpressions[0].(*ast.IntExpression)
		if !ok {
			return
		}
		checker.checkIntegerLiteral(intExpression, integerType)
	}
}

// CompositeType

type CompositeType struct {
	Location     ast.Location
	Kind         common.CompositeKind
	Identifier   string
	Conformances []*InterfaceType
	Members      map[string]*Member
	// TODO: add support for overloaded initializers
	ConstructorParameterTypeAnnotations []*TypeAnnotation
}

func (*CompositeType) isType() {}

func (t *CompositeType) String() string {
	return t.Identifier
}

func (t *CompositeType) Equal(other Type) bool {
	otherStructure, ok := other.(*CompositeType)
	if !ok {
		return false
	}

	return otherStructure.Kind == t.Kind &&
		otherStructure.Identifier == t.Identifier
}

func (t *CompositeType) HasMembers() bool {
	return true
}

func (t *CompositeType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

func (t *CompositeType) IsResourceType() bool {
	return t.Kind == common.CompositeKindResource
}

func (t *CompositeType) IsInvalidType() bool {
	// TODO: maybe if any member has an invalid type?
	return false
}

// Member

type Member struct {
	Type           Type
	VariableKind   ast.VariableKind
	ArgumentLabels []string
}

// NewMemberForType initializes a new member type and panics if the member declaration is invalid.
func NewMemberForType(ty Type, identifier string, member Member) *Member {

	if invokableType, ok := member.Type.(InvokableType); ok {
		functionType := invokableType.InvocationFunctionType()

		if member.ArgumentLabels != nil &&
			len(member.ArgumentLabels) != len(functionType.ParameterTypeAnnotations) {

			panic(fmt.Sprintf(
				"member `%s.%s` has incorrect argument label count",
				ty,
				identifier,
			))
		}
	} else {
		if member.ArgumentLabels != nil {
			panic(fmt.Sprintf(
				"non-function member `%s.%s` should not declare argument labels",
				ty,
				identifier,
			))
		}
	}

	return &member
}

type MemberAccessibleType interface {
	Type
	HasMembers() bool
	GetMember(field string, targetRange ast.Range, report func(error)) *Member
}

// InterfaceType

type InterfaceType struct {
	CompositeKind common.CompositeKind
	Identifier    string
	Members       map[string]*Member
	// TODO: add support for overloaded initializers
	InitializerParameterTypeAnnotations []*TypeAnnotation
}

func (*InterfaceType) isType() {}

func (t *InterfaceType) String() string {
	return t.Identifier
}

func (t *InterfaceType) Equal(other Type) bool {
	otherInterface, ok := other.(*InterfaceType)
	if !ok {
		return false
	}

	return otherInterface.CompositeKind == t.CompositeKind &&
		otherInterface.Identifier == t.Identifier
}

func (t *InterfaceType) HasMembers() bool {
	return true
}

func (t *InterfaceType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

func (t *InterfaceType) IsResourceType() bool {
	return t.CompositeKind == common.CompositeKindResource
}

func (t *InterfaceType) IsInvalidType() bool {
	// TODO: maybe if any member has an invalid type?
	return false
}

// DictionaryType

type DictionaryType struct {
	KeyType   Type
	ValueType Type
}

func (*DictionaryType) isType() {}

func (t *DictionaryType) String() string {
	return fmt.Sprintf("{%s: %s}", t.KeyType, t.ValueType)
}

func (t *DictionaryType) Equal(other Type) bool {
	otherDictionary, ok := other.(*DictionaryType)
	if !ok {
		return false
	}

	return otherDictionary.KeyType.Equal(t.KeyType) &&
		otherDictionary.ValueType.Equal(t.ValueType)
}

func (t *DictionaryType) IsResourceType() bool {
	return t.KeyType.IsResourceType() ||
		t.ValueType.IsResourceType()
}

func (t *DictionaryType) IsInvalidType() bool {
	return t.KeyType.IsInvalidType() ||
		t.ValueType.IsInvalidType()
}

func (t *DictionaryType) HasMembers() bool {
	return true
}

func (t *DictionaryType) GetMember(field string, _ ast.Range, _ func(error)) *Member {
	switch field {
	case "length":
		return NewMemberForType(t, field, Member{
			Type:         &IntType{},
			VariableKind: ast.VariableKindConstant,
		})

	case "insert":
		return NewMemberForType(t, field, Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					t.KeyType,
					t.ValueType,
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&OptionalType{
						Type: t.ValueType,
					},
				),
			},
			ArgumentLabels: []string{"key", ArgumentLabelNotRequired},
		})

	case "remove":
		return NewMemberForType(t, "remove", Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypeAnnotations: NewTypeAnnotations(
					t.KeyType,
				),
				ReturnTypeAnnotation: NewTypeAnnotation(
					&OptionalType{
						Type: t.ValueType,
					},
				),
			},
			ArgumentLabels: []string{"key"},
		})

	default:
		return nil
	}
}

func (t *DictionaryType) isValueIndexableType() bool {
	return true
}

func (t *DictionaryType) ElementType(isAssignment bool) Type {
	return &OptionalType{Type: t.ValueType}
}

func (t *DictionaryType) IndexingType() Type {
	return t.KeyType
}

type DictionaryEntryType struct {
	KeyType   Type
	ValueType Type
}

// StorageType

type StorageType struct{}

func (t *StorageType) isType() {}

func (t *StorageType) String() string {
	return "Storage"
}

func (t *StorageType) Equal(other Type) bool {
	_, ok := other.(*StorageType)
	return ok
}

func (t *StorageType) IsResourceType() bool {
	// NOTE: even though storage may contain resources,
	//   we define it to not behave like a resource
	return false
}

func (t *StorageType) IsInvalidType() bool {
	return false
}

func (t *StorageType) isTypeIndexableType() {}

func (t *StorageType) ElementType(indexingType Type, isAssignment bool) Type {
	// NOTE: like dictionary
	return &OptionalType{Type: indexingType}
}

// EventType

type EventType struct {
	Identifier                          string
	Location                            ast.Location
	Fields                              []EventFieldType
	ConstructorParameterTypeAnnotations []*TypeAnnotation
}

func (*EventType) isType() {}

func (t *EventType) String() string {
	var fields strings.Builder
	for i, field := range t.Fields {
		if i > 0 {
			fields.WriteString(", ")
		}
		fields.WriteString(field.String())
	}

	return fmt.Sprintf("%s(%s)", t.Identifier, fields.String())
}

func (t *EventType) Equal(other Type) bool {
	otherEvent, ok := other.(*EventType)
	if !ok {
		return false
	}

	if t.Identifier != otherEvent.Identifier {
		return false
	}

	if len(t.Fields) != len(otherEvent.Fields) {
		return false
	}

	for i, field := range t.Fields {
		otherField := otherEvent.Fields[i]
		if !field.Equal(otherField) {
			return false
		}
	}

	return true
}

func (t *EventType) ConstructorFunctionType() *FunctionType {
	return &FunctionType{
		ParameterTypeAnnotations: t.ConstructorParameterTypeAnnotations,
		ReturnTypeAnnotation:     NewTypeAnnotation(t),
	}
}

func (*EventType) IsResourceType() bool {
	return false
}

func (*EventType) IsInvalidType() bool {
	return false
}

type EventFieldType struct {
	Identifier string
	Type       Type
}

func (t EventFieldType) String() string {
	return fmt.Sprintf("%s: %s", t.Identifier, t.Type)
}

func (t EventFieldType) Equal(other EventFieldType) bool {
	return t.Identifier == other.Identifier &&
		t.Type.Equal(other.Type)
}

// ReferenceType represents the reference to a value
type ReferenceType struct {
	Type Type
}

func (*ReferenceType) isType() {}

func (t *ReferenceType) String() string {
	if t.Type == nil {
		return "reference"
	}
	return fmt.Sprintf("&%s", t.Type)
}

func (t *ReferenceType) Equal(other Type) bool {
	otherReference, ok := other.(*ReferenceType)
	if !ok {
		return false
	}
	return t.Type.Equal(otherReference.Type)
}

func (t *ReferenceType) IsResourceType() bool {
	return false
}

func (t *ReferenceType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *ReferenceType) HasMembers() bool {
	referencedType, ok := t.Type.(MemberAccessibleType)
	if !ok {
		return false
	}
	return referencedType.HasMembers()
}

func (t *ReferenceType) GetMember(field string, targetRange ast.Range, report func(error)) *Member {
	// forward to referenced type, if it has members
	referencedTypeWithMember, ok := t.Type.(MemberAccessibleType)
	if !ok {
		return nil
	}
	return referencedTypeWithMember.GetMember(field, targetRange, report)
}

func (t *ReferenceType) isValueIndexableType() bool {
	referencedType, ok := t.Type.(ValueIndexableType)
	if !ok {
		return false
	}
	return referencedType.isValueIndexableType()
}

func (t *ReferenceType) ElementType(isAssignment bool) Type {
	referencedType, ok := t.Type.(ValueIndexableType)
	if !ok {
		return nil
	}
	return referencedType.ElementType(isAssignment)
}

func (t *ReferenceType) IndexingType() Type {
	referencedType, ok := t.Type.(ValueIndexableType)
	if !ok {
		return nil
	}
	return referencedType.IndexingType()
}

// AddressType represents the address type
type AddressType struct{}

func (*AddressType) isType() {}

func (*AddressType) String() string {
	return "Address"
}

func (*AddressType) Equal(other Type) bool {
	_, ok := other.(*AddressType)
	return ok
}

func (*AddressType) IsResourceType() bool {
	return false
}

func (*AddressType) IsInvalidType() bool {
	return false
}

var AddressTypeMin = big.NewInt(0)
var AddressTypeMax *big.Int

func init() {
	AddressTypeMax = big.NewInt(2)
	AddressTypeMax.Exp(AddressTypeMax, big.NewInt(160), nil)
	AddressTypeMax.Sub(AddressTypeMax, big.NewInt(1))
}

func (*AddressType) Min() *big.Int {
	return AddressTypeMin
}

func (*AddressType) Max() *big.Int {
	return AddressTypeMax
}

// IsSubType determines if the given subtype is a subtype
// of the given supertype.
//
// Types are subtypes of themselves.
//
func IsSubType(subType Type, superType Type) bool {
	if subType.Equal(superType) {
		return true
	}

	if _, ok := superType.(*AnyType); ok {
		return true
	}

	if _, ok := subType.(*NeverType); ok {
		return true
	}

	switch typedSuperType := superType.(type) {
	case *IntegerType:
		switch subType.(type) {
		case *IntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type,
			*UInt8Type, *UInt16Type, *UInt32Type, *UInt64Type:

			return true

		default:
			return false
		}

	case *CharacterType:
		// TODO: only allow valid character literals
		return subType.Equal(&StringType{})

	case *OptionalType:
		optionalSubType, ok := subType.(*OptionalType)
		if !ok {
			// T <: U? if T <: U
			return IsSubType(subType, typedSuperType.Type)
		}
		// optionals are covariant: T? <: U? if T <: U
		return IsSubType(optionalSubType.Type, typedSuperType.Type)

	case *InterfaceType:
		compositeSubType, ok := subType.(*CompositeType)
		if !ok {
			return false
		}
		// TODO: optimize, use set
		for _, conformance := range compositeSubType.Conformances {
			if typedSuperType.Equal(conformance) {
				return true
			}
		}
		return false

	case *DictionaryType:
		typedSubType, ok := subType.(*DictionaryType)
		if !ok {
			return false
		}

		return IsSubType(typedSubType.KeyType, typedSuperType.KeyType) &&
			IsSubType(typedSubType.ValueType, typedSuperType.ValueType)

	case *VariableSizedType:
		typedSubType, ok := subType.(*VariableSizedType)
		if !ok {
			return false
		}

		return IsSubType(
			typedSubType.ElementType(false),
			typedSuperType.ElementType(false),
		)

	case *ConstantSizedType:
		typedSubType, ok := subType.(*ConstantSizedType)
		if !ok {
			return false
		}

		if typedSubType.Size != typedSuperType.Size {
			return false
		}

		return IsSubType(
			typedSubType.ElementType(false),
			typedSuperType.ElementType(false),
		)

	case *ReferenceType:
		typedSubType, ok := subType.(*ReferenceType)
		if !ok {
			return false
		}

		// references are covariant: &T <: &U if T <: U
		return IsSubType(typedSubType.Type, typedSuperType.Type)
	}

	// TODO: functions

	return false
}

func IsConcatenatableType(ty Type) bool {
	_, isArrayType := ty.(ArrayType)
	return IsSubType(ty, &StringType{}) || isArrayType
}

func IsEquatableType(ty Type) bool {

	// TODO: add support for arrays and dictionaries
	// TODO: add support for composites that are equatable

	if IsSubType(ty, &StringType{}) ||
		IsSubType(ty, &BoolType{}) ||
		IsSubType(ty, &IntegerType{}) {

		return true
	}

	if optionalType, ok := ty.(*OptionalType); ok {
		return IsEquatableType(optionalType.Type)
	}

	return false
}

// UnwrapOptionalType returns the type if it is not an optional type,
// or the inner-most type if it is (optional types are repeatedly unwrapped)
//
func UnwrapOptionalType(ty Type) Type {
	for {
		optionalType, ok := ty.(*OptionalType)
		if !ok {
			return ty
		}
		ty = optionalType.Type
	}
}

func AreCompatibleEquatableTypes(leftType, rightType Type) bool {
	unwrappedLeftType := UnwrapOptionalType(leftType)
	unwrappedRightType := UnwrapOptionalType(rightType)

	leftIsEquatable := IsEquatableType(unwrappedLeftType)
	rightIsEquatable := IsEquatableType(unwrappedRightType)

	if unwrappedLeftType.Equal(unwrappedRightType) &&
		leftIsEquatable && rightIsEquatable {

		return true
	}

	// The types are equatable if this is a comparison with `nil`,
	// which has type `Never?`

	if IsNilType(leftType) || IsNilType(rightType) {
		return true
	}

	return false
}

// IsNilType returns true if the given type is the type of `nil`, i.e. `Never?`.
//
func IsNilType(ty Type) bool {
	optionalType, ok := ty.(*OptionalType)
	if !ok {
		return false
	}

	if _, ok := optionalType.Type.(*NeverType); !ok {
		return false
	}

	return true
}
