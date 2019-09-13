package ast

import "github.com/dapperlabs/flow-go/pkg/language/runtime/errors"

//go:generate stringer -type=VariableKind

type VariableKind int

const (
	VariableKindNotSpecified VariableKind = iota
	VariableKindVariable
	VariableKindConstant
)

func (k VariableKind) Name() string {
	switch k {
	case VariableKindVariable:
		return "variable"
	case VariableKindConstant:
		return "constant"
	}

	panic(&errors.UnreachableError{})
}
