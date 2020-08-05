/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package interpreter

import (
	"github.com/onflow/cadence-initial/runtime/errors"
	"github.com/onflow/cadence-initial/runtime/sema"
)

//go:generate stringer -type=PrimitiveStaticType -trimprefix=PrimitiveStaticType

// PrimitiveStaticType

type PrimitiveStaticType uint

// !!! *WARNING* !!!
//
// Only add new types by:
// - replacing existing placeholders (`_`) with new types
// - appending new types
//
// Only remove types by:
// - replace existing types with a placeholder `_`
//
// DO *NOT* REPLACE EXISTING TYPES!
// DO *NOT* ADD NEW TYPES IN BETWEEN!

const (
	PrimitiveStaticTypeUnknown PrimitiveStaticType = iota
	PrimitiveStaticTypeVoid
	PrimitiveStaticTypeAny
	PrimitiveStaticTypeNever
	PrimitiveStaticTypeAnyStruct
	PrimitiveStaticTypeAnyResource
	PrimitiveStaticTypeBool
	PrimitiveStaticTypeAddress
	PrimitiveStaticTypeString
	PrimitiveStaticTypeCharacter
	_
	_
	_
	_
	_
	_
	_
	_

	// Number
	PrimitiveStaticTypeNumber
	PrimitiveStaticTypeSignedNumber
	_
	_
	_
	_

	// Integer
	PrimitiveStaticTypeInteger
	PrimitiveStaticTypeSignedInteger
	_
	_
	_
	_

	// FixedPoint
	PrimitiveStaticTypeFixedPoint
	PrimitiveStaticTypeSignedFixedPoint
	_
	_
	_
	_

	// Int*
	PrimitiveStaticTypeInt
	PrimitiveStaticTypeInt8
	PrimitiveStaticTypeInt16
	PrimitiveStaticTypeInt32
	PrimitiveStaticTypeInt64
	PrimitiveStaticTypeInt128
	PrimitiveStaticTypeInt256
	_

	// UInt*
	PrimitiveStaticTypeUInt
	PrimitiveStaticTypeUInt8
	PrimitiveStaticTypeUInt16
	PrimitiveStaticTypeUInt32
	PrimitiveStaticTypeUInt64
	PrimitiveStaticTypeUInt128
	PrimitiveStaticTypeUInt256
	_

	// Word *
	_
	PrimitiveStaticTypeWord8
	PrimitiveStaticTypeWord16
	PrimitiveStaticTypeWord32
	PrimitiveStaticTypeWord64
	_ // future: Word128
	_ // future: Word256
	_

	// Fix*
	_
	_ // future: Fix8
	_ // future: Fix16
	_ // future: Fix32
	PrimitiveStaticTypeFix64
	_ // future: Fix128
	_ // future: Fix256
	_

	// UFix*
	_
	_ // future: UFix8
	_ // future: UFix16
	_ // future: UFix32
	PrimitiveStaticTypeUFix64
	_ // future: UFix128
	_ // future: UFix256
	_

	// Storage

	PrimitiveStaticTypePath
	PrimitiveStaticTypeCapability
)

func (PrimitiveStaticType) isStaticType() {}

func (i PrimitiveStaticType) SemaType() sema.Type {
	switch i {
	case PrimitiveStaticTypeVoid:
		return &sema.VoidType{}

	case PrimitiveStaticTypeAny:
		return &sema.AnyType{}

	case PrimitiveStaticTypeNever:
		return &sema.NeverType{}

	case PrimitiveStaticTypeAnyStruct:
		return &sema.AnyStructType{}

	case PrimitiveStaticTypeAnyResource:
		return &sema.AnyResourceType{}

	case PrimitiveStaticTypeBool:
		return &sema.BoolType{}

	case PrimitiveStaticTypeAddress:
		return &sema.AddressType{}

	case PrimitiveStaticTypeString:
		return &sema.StringType{}

	case PrimitiveStaticTypeCharacter:
		return &sema.CharacterType{}

	// Number

	case PrimitiveStaticTypeNumber:
		return &sema.NumberType{}
	case PrimitiveStaticTypeSignedNumber:
		return &sema.SignedNumberType{}

	// Integer
	case PrimitiveStaticTypeInteger:
		return &sema.IntegerType{}
	case PrimitiveStaticTypeSignedInteger:
		return &sema.SignedIntegerType{}

	// FixedPoint
	case PrimitiveStaticTypeFixedPoint:
		return &sema.FixedPointType{}
	case PrimitiveStaticTypeSignedFixedPoint:
		return &sema.SignedFixedPointType{}

	// Int*
	case PrimitiveStaticTypeInt:
		return &sema.IntType{}
	case PrimitiveStaticTypeInt8:
		return &sema.Int8Type{}
	case PrimitiveStaticTypeInt16:
		return &sema.Int16Type{}
	case PrimitiveStaticTypeInt32:
		return &sema.Int32Type{}
	case PrimitiveStaticTypeInt64:
		return &sema.Int64Type{}
	case PrimitiveStaticTypeInt128:
		return &sema.Int128Type{}
	case PrimitiveStaticTypeInt256:
		return &sema.Int256Type{}

	// UInt*
	case PrimitiveStaticTypeUInt:
		return &sema.UIntType{}
	case PrimitiveStaticTypeUInt8:
		return &sema.UInt8Type{}
	case PrimitiveStaticTypeUInt16:
		return &sema.UInt16Type{}
	case PrimitiveStaticTypeUInt32:
		return &sema.UInt32Type{}
	case PrimitiveStaticTypeUInt64:
		return &sema.UInt64Type{}
	case PrimitiveStaticTypeUInt128:
		return &sema.UInt128Type{}
	case PrimitiveStaticTypeUInt256:
		return &sema.UInt256Type{}

	// Word *

	case PrimitiveStaticTypeWord8:
		return &sema.Word8Type{}
	case PrimitiveStaticTypeWord16:
		return &sema.Word16Type{}
	case PrimitiveStaticTypeWord32:
		return &sema.Word32Type{}
	case PrimitiveStaticTypeWord64:
		return &sema.Word64Type{}

	// Fix*
	case PrimitiveStaticTypeFix64:
		return &sema.Fix64Type{}

	// UFix*
	case PrimitiveStaticTypeUFix64:
		return &sema.UFix64Type{}

	// Storage

	case PrimitiveStaticTypePath:
		return &sema.PathType{}
	case PrimitiveStaticTypeCapability:
		return &sema.CapabilityType{}

	default:
		panic(errors.NewUnreachableError())
	}
}

// ConvertSemaToPrimitiveStaticType converts a `sema.Type` to a `PrimitiveStaticType`.
//
// Returns `PrimitiveStaticTypeUnknown` if the given type is not a primitive type.
//
func ConvertSemaToPrimitiveStaticType(t sema.Type) PrimitiveStaticType {
	switch t.(type) {
	case *sema.VoidType:
		return PrimitiveStaticTypeVoid

	case *sema.AnyType:
		return PrimitiveStaticTypeAny

	case *sema.NeverType:
		return PrimitiveStaticTypeNever

	case *sema.AnyStructType:
		return PrimitiveStaticTypeAnyStruct

	case *sema.AnyResourceType:
		return PrimitiveStaticTypeAnyResource

	case *sema.BoolType:
		return PrimitiveStaticTypeBool

	case *sema.AddressType:
		return PrimitiveStaticTypeAddress

	case *sema.StringType:
		return PrimitiveStaticTypeString

	case *sema.CharacterType:
		return PrimitiveStaticTypeCharacter

	// Number

	case *sema.NumberType:
		return PrimitiveStaticTypeNumber
	case *sema.SignedNumberType:
		return PrimitiveStaticTypeSignedNumber

	// Integer
	case *sema.IntegerType:
		return PrimitiveStaticTypeInteger
	case *sema.SignedIntegerType:
		return PrimitiveStaticTypeSignedInteger

	// FixedPoint
	case *sema.FixedPointType:
		return PrimitiveStaticTypeFixedPoint
	case *sema.SignedFixedPointType:
		return PrimitiveStaticTypeSignedFixedPoint

	// Int*
	case *sema.IntType:
		return PrimitiveStaticTypeInt
	case *sema.Int8Type:
		return PrimitiveStaticTypeInt8
	case *sema.Int16Type:
		return PrimitiveStaticTypeInt16
	case *sema.Int32Type:
		return PrimitiveStaticTypeInt32
	case *sema.Int64Type:
		return PrimitiveStaticTypeInt64
	case *sema.Int128Type:
		return PrimitiveStaticTypeInt128
	case *sema.Int256Type:
		return PrimitiveStaticTypeInt256

	// UInt*
	case *sema.UIntType:
		return PrimitiveStaticTypeUInt
	case *sema.UInt8Type:
		return PrimitiveStaticTypeUInt8
	case *sema.UInt16Type:
		return PrimitiveStaticTypeUInt16
	case *sema.UInt32Type:
		return PrimitiveStaticTypeUInt32
	case *sema.UInt64Type:
		return PrimitiveStaticTypeUInt64
	case *sema.UInt128Type:
		return PrimitiveStaticTypeUInt128
	case *sema.UInt256Type:
		return PrimitiveStaticTypeUInt256

	// Word *

	case *sema.Word8Type:
		return PrimitiveStaticTypeWord8
	case *sema.Word16Type:
		return PrimitiveStaticTypeWord16
	case *sema.Word32Type:
		return PrimitiveStaticTypeWord32
	case *sema.Word64Type:
		return PrimitiveStaticTypeWord64

	// Fix*
	case *sema.Fix64Type:
		return PrimitiveStaticTypeFix64

	// UFix*
	case *sema.UFix64Type:
		return PrimitiveStaticTypeUFix64

	// Storage

	case *sema.PathType:
		return PrimitiveStaticTypePath
	case *sema.CapabilityType:
		return PrimitiveStaticTypeCapability

	default:
		return PrimitiveStaticTypeUnknown
	}
}
