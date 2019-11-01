package ast

import "github.com/dapperlabs/flow-go/language/runtime/errors"

//go:generate stringer -type=TransferOperation

type TransferOperation int

const (
	TransferOperationUnknown TransferOperation = iota
	TransferOperationCopy
	TransferOperationMove
)

func (k TransferOperation) Operator() string {
	switch k {
	case TransferOperationCopy:
		return "="
	case TransferOperationMove:
		return "<-"
	}

	panic(&errors.UnreachableError{})
}
