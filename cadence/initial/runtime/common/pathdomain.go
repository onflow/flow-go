/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"github.com/onflow/cadence-initial/runtime/errors"
)

//go:generate stringer -type=PathDomain

type PathDomain uint

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

func PathDomainFromIdentifier(domain string) PathDomain {
	result, ok := AllPathDomainsByIdentifier[domain]
	if !ok {
		return PathDomainUnknown
	}
	return result
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
