package interpreter

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=AccessLevel

type AccessLevel int

const (
	AccessLevelPrivate AccessLevel = iota
	AccessLevelPublic
)

func (k AccessLevel) Prefix() string {
	switch k {
	case AccessLevelPrivate:
		return "private"
	case AccessLevelPublic:
		return "public"
	default:
		panic(errors.NewUnreachableError())
	}
}
