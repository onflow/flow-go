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
	"github.com/onflow/cadence-initial/runtime/sema"
)

type DynamicType interface {
	IsDynamicType()
}

type ReferenceDynamicType interface {
	DynamicType
	isReferenceType()
	Authorized() bool
	InnerType() DynamicType
}

// MetaTypeDynamicType

type MetaTypeDynamicType struct{}

func (MetaTypeDynamicType) IsDynamicType() {}

// VoidDynamicType

type VoidDynamicType struct{}

func (VoidDynamicType) IsDynamicType() {}

// StringDynamicType

type StringDynamicType struct{}

func (StringDynamicType) IsDynamicType() {}

// BoolDynamicType

type BoolDynamicType struct{}

func (BoolDynamicType) IsDynamicType() {}

// ArrayDynamicType

type ArrayDynamicType struct {
	ElementTypes []DynamicType
}

func (ArrayDynamicType) IsDynamicType() {}

// NumberDynamicType

type NumberDynamicType struct {
	StaticType sema.Type
}

func (NumberDynamicType) IsDynamicType() {}

// CompositeDynamicType

type CompositeDynamicType struct {
	StaticType sema.Type
}

func (CompositeDynamicType) IsDynamicType() {}

// DictionaryDynamicType

type DictionaryDynamicType struct {
	EntryTypes []struct{ KeyType, ValueType DynamicType }
}

func (DictionaryDynamicType) IsDynamicType() {}

// NilDynamicType

type NilDynamicType struct{}

func (NilDynamicType) IsDynamicType() {}

// SomeDynamicType

type SomeDynamicType struct {
	InnerType DynamicType
}

func (SomeDynamicType) IsDynamicType() {}

// StorageReferenceDynamicType

type StorageReferenceDynamicType struct {
	authorized bool
	innerType  DynamicType
}

func (StorageReferenceDynamicType) IsDynamicType() {}

func (StorageReferenceDynamicType) isReferenceType() {}

func (t StorageReferenceDynamicType) Authorized() bool {
	return t.authorized
}

func (t StorageReferenceDynamicType) InnerType() DynamicType {
	return t.innerType
}

// EphemeralReferenceDynamicType

type EphemeralReferenceDynamicType struct {
	authorized bool
	innerType  DynamicType
}

func (EphemeralReferenceDynamicType) IsDynamicType() {}

func (EphemeralReferenceDynamicType) isReferenceType() {}

func (t EphemeralReferenceDynamicType) Authorized() bool {
	return t.authorized
}

func (t EphemeralReferenceDynamicType) InnerType() DynamicType {
	return t.innerType
}

// AddressDynamicType

type AddressDynamicType struct{}

func (AddressDynamicType) IsDynamicType() {}

// PublishedDynamicType

type PublishedDynamicType struct{}

func (PublishedDynamicType) IsDynamicType() {}

// FunctionDynamicType

type FunctionDynamicType struct{}

func (FunctionDynamicType) IsDynamicType() {}

// PathDynamicType

type PathDynamicType struct{}

func (PathDynamicType) IsDynamicType() {}

// CapabilityDynamicType

type CapabilityDynamicType struct{}

func (CapabilityDynamicType) IsDynamicType() {}

// AuthAccountDynamicType

type AuthAccountDynamicType struct{}

func (AuthAccountDynamicType) IsDynamicType() {}

// PublicAccountDynamicType

type PublicAccountDynamicType struct{}

func (PublicAccountDynamicType) IsDynamicType() {}
