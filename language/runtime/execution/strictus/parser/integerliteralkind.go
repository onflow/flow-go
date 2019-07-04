package parser

import "bamboo-runtime/execution/strictus/errors"

//go:generate stringer -type=IntegerLiteralKind

type IntegerLiteralKind int

const (
	IntegerLiteralKindUnknown IntegerLiteralKind = iota
	IntegerLiteralKindBinary
	IntegerLiteralKindOctal
	IntegerLiteralKindDecimal
	IntegerLiteralKindHexadecimal
)

func (k IntegerLiteralKind) Base() int {
	switch k {
	case IntegerLiteralKindBinary:
		return 2
	case IntegerLiteralKindOctal:
		return 8
	case IntegerLiteralKindDecimal:
		return 10
	case IntegerLiteralKindHexadecimal:
		return 16
	}

	panic(errors.UnreachableError{})
}

func (k IntegerLiteralKind) Name() string {
	switch k {
	case IntegerLiteralKindUnknown:
		return "unknown"
	case IntegerLiteralKindBinary:
		return "binary"
	case IntegerLiteralKindOctal:
		return "octal"
	case IntegerLiteralKindDecimal:
		return "decimal"
	case IntegerLiteralKindHexadecimal:
		return "hexadecimal"
	}

	panic(errors.UnreachableError{})
}
