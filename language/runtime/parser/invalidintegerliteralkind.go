package parser

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=InvalidIntegerLiteralKind

type InvalidIntegerLiteralKind int

const (
	InvalidIntegerLiteralKindUnknown InvalidIntegerLiteralKind = iota
	InvalidIntegerLiteralKindLeadingUnderscore
	InvalidIntegerLiteralKindTrailingUnderscore
	InvalidIntegerLiteralKindUnknownPrefix
)

func (k InvalidIntegerLiteralKind) Description() string {
	switch k {
	case InvalidIntegerLiteralKindLeadingUnderscore:
		return "leading underscore"
	case InvalidIntegerLiteralKindTrailingUnderscore:
		return "trailing underscore"
	case InvalidIntegerLiteralKindUnknownPrefix:
		return "unknown prefix"
	}

	panic(&errors.UnreachableError{})
}
