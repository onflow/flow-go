package ast

import "github.com/dapperlabs/flow-go/language/runtime/errors"

//go:generate stringer -type=Access

type Access int

const (
	AccessNotSpecified Access = iota
	AccessPublic
	AccessPublicSettable
)

func (s Access) Keyword() string {
	switch s {
	case AccessNotSpecified:
		return ""
	case AccessPublic:
		return "pub"
	case AccessPublicSettable:
		return "pub(set)"
	}

	panic(&errors.UnreachableError{})
}
