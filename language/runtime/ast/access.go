package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=Access

type Access int

const (
	AccessNotSpecified Access = iota
	AccessPrivate
	AccessPublic
	AccessPublicSettable
)

var Accesses = []Access{
	AccessNotSpecified,
	AccessPrivate,
	AccessPublic,
	AccessPublicSettable,
}

func (s Access) Keyword() string {
	switch s {
	case AccessNotSpecified:
		return ""
	case AccessPrivate:
		return "priv"
	case AccessPublic:
		return "pub"
	case AccessPublicSettable:
		return "pub(set)"
	}

	panic(errors.NewUnreachableError())
}

func (s Access) Description() string {
	switch s {
	case AccessNotSpecified:
		return "not specified"
	case AccessPrivate:
		return "private"
	case AccessPublic:
		return "public"
	case AccessPublicSettable:
		return "public settable"
	}

	panic(errors.NewUnreachableError())
}
