package ast

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=Access

type Access int

// NOTE: order indicates permissiveness: from less to most permissive!

const (
	AccessNotSpecified Access = iota
	AccessPrivate
	AccessContract
	AccessAccount
	AccessPublic
	AccessPublicSettable
)

func (a Access) IsLessPermissiveThan(otherAccess Access) bool {
	return a < otherAccess
}

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
	case AccessAccount:
		return "access(account)"
	case AccessContract:
		return "access(contract)"
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
	case AccessAccount:
		return "account"
	case AccessContract:
		return "contract"
	}

	panic(errors.NewUnreachableError())
}
