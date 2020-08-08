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

//go:generate stringer -type=DeclarationKind

type DeclarationKind uint

const (
	DeclarationKindUnknown DeclarationKind = iota
	DeclarationKindValue
	DeclarationKindFunction
	DeclarationKindVariable
	DeclarationKindConstant
	DeclarationKindType
	DeclarationKindParameter
	DeclarationKindArgumentLabel
	DeclarationKindStructure
	DeclarationKindResource
	DeclarationKindContract
	DeclarationKindEvent
	DeclarationKindField
	DeclarationKindInitializer
	DeclarationKindDestructor
	DeclarationKindStructureInterface
	DeclarationKindResourceInterface
	DeclarationKindContractInterface
	DeclarationKindImport
	DeclarationKindSelf
	DeclarationKindResult
	DeclarationKindTransaction
	DeclarationKindPrepare
	DeclarationKindExecute
	DeclarationKindTypeParameter
)

func (k DeclarationKind) IsTypeDeclaration() bool {
	switch k {
	case DeclarationKindStructure,
		DeclarationKindResource,
		DeclarationKindContract,
		DeclarationKindEvent,
		DeclarationKindStructureInterface,
		DeclarationKindResourceInterface,
		DeclarationKindContractInterface,
		DeclarationKindTypeParameter:

		return true

	default:
		return false
	}
}

func (k DeclarationKind) Name() string {
	switch k {
	case DeclarationKindValue:
		return "value"
	case DeclarationKindFunction:
		return "function"
	case DeclarationKindVariable:
		return "variable"
	case DeclarationKindConstant:
		return "constant"
	case DeclarationKindType:
		return "type"
	case DeclarationKindParameter:
		return "parameter"
	case DeclarationKindArgumentLabel:
		return "argument label"
	case DeclarationKindStructure:
		return "structure"
	case DeclarationKindResource:
		return "resource"
	case DeclarationKindContract:
		return "contract"
	case DeclarationKindEvent:
		return "event"
	case DeclarationKindField:
		return "field"
	case DeclarationKindInitializer:
		return "initializer"
	case DeclarationKindDestructor:
		return "destructor"
	case DeclarationKindStructureInterface:
		return "structure interface"
	case DeclarationKindResourceInterface:
		return "resource interface"
	case DeclarationKindContractInterface:
		return "contract interface"
	case DeclarationKindImport:
		return "import"
	case DeclarationKindSelf:
		return "self"
	case DeclarationKindResult:
		return "result"
	case DeclarationKindTransaction:
		return "transaction"
	case DeclarationKindPrepare:
		return "prepare"
	case DeclarationKindExecute:
		return "execute"
	case DeclarationKindTypeParameter:
		return "type parameter"
	case DeclarationKindUnknown:
		return "unknown"
	}

	panic(errors.NewUnreachableError())
}

func (k DeclarationKind) Keywords() string {
	switch k {
	case DeclarationKindFunction:
		return "fun"
	case DeclarationKindVariable:
		return "var"
	case DeclarationKindConstant:
		return "const"
	case DeclarationKindStructure:
		return "struct"
	case DeclarationKindResource:
		return "resource"
	case DeclarationKindContract:
		return "contract"
	case DeclarationKindEvent:
		return "event"
	case DeclarationKindInitializer:
		return "init"
	case DeclarationKindDestructor:
		return "destroy"
	case DeclarationKindStructureInterface:
		return "struct interface"
	case DeclarationKindResourceInterface:
		return "resource interface"
	case DeclarationKindContractInterface:
		return "contract interface"
	case DeclarationKindImport:
		return "import"
	case DeclarationKindSelf:
		return "self"
	case DeclarationKindResult:
		return "result"
	case DeclarationKindTransaction:
		return "transaction"
	case DeclarationKindPrepare:
		return "prepare"
	case DeclarationKindExecute:
		return "execute"
	default:
		return ""
	}
}
