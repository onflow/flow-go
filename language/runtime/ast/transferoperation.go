package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=TransferOperation

type TransferOperation int

const (
	TransferOperationUnknown TransferOperation = iota
	TransferOperationCopy
	TransferOperationMove
	TransferOperationMoveForced
)

func (k TransferOperation) Operator() string {
	switch k {
	case TransferOperationCopy:
		return "="
	case TransferOperationMove:
		return "<-"
	case TransferOperationMoveForced:
		return "<-!"
	}

	panic(errors.NewUnreachableError())
}

func (k TransferOperation) IsMove() bool {
	switch k {
	case TransferOperationMove, TransferOperationMoveForced:
		return true
	}

	return false
}
