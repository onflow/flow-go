package common

import (
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=PathDomain

type PathDomain int

const (
	PathDomainUnknown PathDomain = iota
	PathDomainStorage
	PathDomainPrivate
	PathDomainPublic
)

var AllPathDomainsByIdentifier = map[string]PathDomain{}

func init() {
	for _, pathDomain := range []PathDomain{
		PathDomainStorage,
		PathDomainPrivate,
		PathDomainPublic,
	} {
		identifier := pathDomain.Identifier()
		AllPathDomainsByIdentifier[identifier] = pathDomain
	}
}

func (i PathDomain) Identifier() string {
	switch i {
	case PathDomainStorage:
		return "storage"

	case PathDomainPrivate:
		return "private"

	case PathDomainPublic:
		return "public"
	}

	panic(errors.NewUnreachableError())
}

func (i PathDomain) Name() string {
	switch i {
	case PathDomainStorage:
		return "storage"

	case PathDomainPrivate:
		return "private"

	case PathDomainPublic:
		return "public"
	}

	panic(errors.NewUnreachableError())
}
