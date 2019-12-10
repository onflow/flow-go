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
)

var CompositeKinds = []CompositeKind{
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
	}

	panic(errors.NewUnreachableError())
}

func (k CompositeKind) Annotation() string {
	if k != CompositeKindResource {
		return ""
	}
	return "@"
}

func (k CompositeKind) AssignmentOperator() string {
	if k != CompositeKindResource {
		return "="
	}
	return "<-"
}

func (k CompositeKind) TransferOperator() string {
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
