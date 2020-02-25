package common

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=CompositeKind

type CompositeKind int

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
