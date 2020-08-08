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

package cadence

import (
	"unicode/utf8"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/parser"
	"example.com/cadence-initial/runtime/sema"
)

func Fuzz(data []byte) int {

	if !utf8.Valid(data) {
		return 0
	}

	program, _, err := parser.ParseProgram(string(data))

	if err != nil {
		return 0
	}

	checker, err := sema.NewChecker(
		program,
		ast.StringLocation("test"),
		sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
	)
	if err != nil {
		return 0
	}

	err = checker.Check()
	if err != nil {
		return 0
	}

	return 1
}
