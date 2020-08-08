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

package parser2

import (
	"fmt"
	"strings"

	"example.com/cadence-initial/runtime/parser2/lexer"
)

const blockCommentStart = "/*"
const blockCommentEnd = "*/"

func (p *parser) parseCommentContent() (comment string) {
	var builder strings.Builder
	defer func() {
		comment = builder.String()
	}()

	builder.WriteString(blockCommentStart)

	var t trampoline
	t = func(builder *strings.Builder) trampoline {
		return func() []trampoline {

			for {
				p.next()

				switch p.current.Type {
				case lexer.TokenEOF:
					p.report(fmt.Errorf(
						"missing comment end %q",
						lexer.TokenBlockCommentEnd,
					))
					return nil

				case lexer.TokenBlockCommentContent:
					builder.WriteString(p.current.Value.(string))

				case lexer.TokenBlockCommentEnd:
					builder.WriteString(blockCommentEnd)
					// Skip the comment end (`*/`)
					p.next()
					return nil

				case lexer.TokenBlockCommentStart:
					builder.WriteString(blockCommentStart)
					// parse inner content, then rest of this comment
					return []trampoline{t, t}

				default:
					p.report(fmt.Errorf(
						"unexpected token in comment: %q",
						p.current.Type,
					))
					return nil
				}
			}
		}
	}(&builder)
	runTrampoline(t)
	return
}
