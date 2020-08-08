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

package common

import (
	"example.com/cadence-initial/runtime/errors"
)

//go:generate stringer -type=CompositeKind

type CompositeKind uint

const (
	CompositeKindUnknown CompositeKind = iota
	CompositeKindStructure
	CompositeKindResource
	CompositeKindContract
	CompositeKindEvent
)

var AllCompositeKinds = []CompositeKind{
	CompositeKindStructure,
	CompositeKindResource,
	CompositeKindContract,
	CompositeKindEvent,
}

var CompositeKindsWithBody = []CompositeKind{
	CompositeKindStructure,
	CompositeKindResource,
	CompositeKindContract,
}

func (k CompositeKind) Name() string {
	switch k {
	case CompositeKindStructure:
		return "structure"
	case CompositeKindResource:
		return "resource"
	case CompositeKindContract:
		return "contract"
	case CompositeKindEvent:
		return "event"
	}

	panic(errors.NewUnreachableError())
}

func (k CompositeKind) Keyword() string {
	switch k {
	case CompositeKindStructure:
		return "struct"
	case CompositeKindResource:
		return "resource"
	case CompositeKindContract:
		return "contract"
	case CompositeKindEvent:
		return "event"
	}

	panic(errors.NewUnreachableError())
}

func (k CompositeKind) DeclarationKind(isInterface bool) DeclarationKind {
	switch k {
	case CompositeKindStructure:
		if isInterface {
			return DeclarationKindStructureInterface
		}
		return DeclarationKindStructure

	case CompositeKindResource:
		if isInterface {
			return DeclarationKindResourceInterface
		}
		return DeclarationKindResource

	case CompositeKindContract:
		if isInterface {
			return DeclarationKindContractInterface
		}
		return DeclarationKindContract

	case CompositeKindEvent:
		if isInterface {
			return DeclarationKindUnknown
		}
		return DeclarationKindEvent
	}

	panic(errors.NewUnreachableError())
}

func (k CompositeKind) Annotation() string {
	if k != CompositeKindResource {
		return ""
	}
	return "@"
}

func (k CompositeKind) TransferOperator() string {
	if k != CompositeKindResource {
		return "="
	}
	return "<-"
}

func (k CompositeKind) MoveOperator() string {
	if k != CompositeKindResource {
		return ""
	}
	return "<-"
}

func (k CompositeKind) ConstructionKeyword() string {
	if k != CompositeKindResource {
		return ""
	}
	return "create"
}

func (k CompositeKind) DestructionKeyword() interface{} {
	if k != CompositeKindResource {
		return ""
	}
	return "destroy"
}

func (k CompositeKind) SupportsInterfaces() bool {
	switch k {
	case CompositeKindStructure,
		CompositeKindResource,
		CompositeKindContract:

		return true

	case CompositeKindEvent:
		return false
	}

	panic(errors.NewUnreachableError())
}
