package parser

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=InvalidNumberLiteralKind

type InvalidNumberLiteralKind int

const (
	InvalidNumberLiteralKindUnknown InvalidNumberLiteralKind = iota
	InvalidNumberLiteralKindLeadingUnderscore
	InvalidNumberLiteralKindTrailingUnderscore
	InvalidNumberLiteralKindUnknownPrefix
)

func (k InvalidNumberLiteralKind) Description() string {
	switch k {
	case InvalidNumberLiteralKindLeadingUnderscore:
		return "leading underscore"
	case InvalidNumberLiteralKindTrailingUnderscore:
		return "trailing underscore"
	case InvalidNumberLiteralKindUnknownPrefix:
		return "unknown prefix"
	}

	panic(errors.NewUnreachableError())
}
