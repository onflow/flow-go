package common

import "github.com/dapperlabs/flow-go/language/runtime/errors"

//go:generate stringer -type=ExpressionKind

type ExpressionKind int

const (
	ExpressionKindUnknown ExpressionKind = iota
	ExpressionKindCreate
	ExpressionKindDestroy
)

func (k ExpressionKind) Name() string {
	switch k {
	case ExpressionKindCreate:
		return "create"
	case ExpressionKindDestroy:
		return "destroy"
	}

	panic(&errors.UnreachableError{})
}
