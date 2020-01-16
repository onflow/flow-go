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

func (a Access) Keyword() string {
	switch a {
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

func (a Access) Description() string {
	switch a {
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

func (a Access) IsLessPermissiveThan(otherAccess Access) bool {
	return a < otherAccess
}
