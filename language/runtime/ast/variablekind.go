package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=VariableKind

type VariableKind int

const (
	VariableKindNotSpecified VariableKind = iota
	VariableKindVariable
	VariableKindConstant
)

var VariableKinds = []VariableKind{
	VariableKindConstant,
	VariableKindVariable,
}

func (k VariableKind) Name() string {
	switch k {
	case VariableKindVariable:
		return "variable"
	case VariableKindConstant:
		return "constant"
	}

	panic(errors.NewUnreachableError())
}

func (k VariableKind) Keyword() string {
	switch k {
	case VariableKindVariable:
		return "var"
	case VariableKindConstant:
		return "let"
	}

	panic(errors.NewUnreachableError())
}
