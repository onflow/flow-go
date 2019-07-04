package parser

import (
	"bamboo-runtime/execution/strictus/errors"
)

//go:generate stringer -type=InvalidIntegerLiteralKind

type InvalidIntegerLiteralKind int

const (
	InvalidIntegerLiteralKindLeadingUnderscore InvalidIntegerLiteralKind = iota
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
