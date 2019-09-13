package common

import "github.com/dapperlabs/flow-go/pkg/language/runtime/errors"

//go:generate stringer -type=OperandSide

type OperandSide int

const (
	OperandSideUnknown OperandSide = iota
	OperandSideLeft
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
