package common

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=DeclarationKind

type DeclarationKind int

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
	case DeclarationKindTypeParameter:
		return ""
	}

	panic(errors.NewUnreachableError())
}
