package interpreter

import "github.com/dapperlabs/bamboo-node/language/runtime/errors"

//go:generate stringer -type=OperandSide

type OperandSide int

const (
	OperandSideLeft OperandSide = iota
	OperandSideRight
)

func (s OperandSide) Name() string {
	switch s {
	case OperandSideLeft:
		return "left"
	case OperandSideRight:
		return "right"
	}

	panic(&errors.UnreachableError{})
}
