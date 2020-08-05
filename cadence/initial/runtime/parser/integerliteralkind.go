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

package parser

import (
	"github.com/onflow/cadence/runtime/errors"
)

//go:generate stringer -type=IntegerLiteralKind

type IntegerLiteralKind uint

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

	panic(errors.NewUnreachableError())
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

	panic(errors.NewUnreachableError())
}
