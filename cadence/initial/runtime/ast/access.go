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

package ast

import (
	"example.com/cadence-initial/runtime/errors"
)

//go:generate stringer -type=Access

type Access uint

// NOTE: order indicates permissiveness: from least to most permissive!

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

// TODO: remove.
//   only used by tests which are not updated yet
//   to include contract and account access

var BasicAccesses = []Access{
	AccessNotSpecified,
	AccessPrivate,
	AccessPublic,
	AccessPublicSettable,
}

var AllAccesses = append(BasicAccesses[:],
	AccessContract,
	AccessAccount,
)

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
