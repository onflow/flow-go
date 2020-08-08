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
	"github.com/antlr/antlr4/runtime/Go/antlr"

	"example.com/cadence-initial/runtime/ast"
)

func PositionFromToken(token antlr.Token) ast.Position {
	return ast.Position{
		Offset: token.GetStart(),
		Line:   token.GetLine(),
		Column: token.GetColumn(),
	}
}

func PositionRangeFromContext(ctx antlr.ParserRuleContext) (start, end ast.Position) {
	start = PositionFromToken(ctx.GetStart())
	end = PositionFromToken(ctx.GetStop())
	return start, end
}
