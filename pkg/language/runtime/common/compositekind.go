package common

import "github.com/dapperlabs/flow-go/pkg/language/runtime/errors"

//go:generate stringer -type=CompositeKind

type CompositeKind int

const (
	CompositeKindUnknown CompositeKind = iota
	CompositeKindStructure
	CompositeKindResource
	CompositeKindContract
	CompositeKindEvent
)

var CompositeKinds = []CompositeKind{
	CompositeKindStructure,
	CompositeKindResource,
	CompositeKindContract,
	CompositeKindEvent,
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

	panic(&errors.UnreachableError{})
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

	panic(&errors.UnreachableError{})
}

func (k CompositeKind) Annotation() string {
	if k != CompositeKindResource {
		return ""
	}
	return "<-"
}

func (k CompositeKind) TransferOperator() string {
	if k != CompositeKindResource {
		return "="
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
