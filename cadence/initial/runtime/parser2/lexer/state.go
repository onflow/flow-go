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

package lexer

import (
	"fmt"
)

const keywordAs = "as"

type stateFn func(*lexer) stateFn

func rootState(l *lexer) stateFn {
	for {
		r := l.next()
		switch r {
		case EOF:
			l.emitType(TokenEOF)
			return nil
		case '+':
			l.emitType(TokenPlus)
		case '-':
			l.emitType(TokenMinus)
		case '*':
			l.emitType(TokenStar)
		case '%':
			l.emitType(TokenPercent)
		case '(':
			l.emitType(TokenParenOpen)
		case ')':
			l.emitType(TokenParenClose)
		case '{':
			l.emitType(TokenBraceOpen)
		case '}':
			l.emitType(TokenBraceClose)
		case '[':
			l.emitType(TokenBracketOpen)
		case ']':
			l.emitType(TokenBracketClose)
		case ',':
			l.emitType(TokenComma)
		case ';':
			l.emitType(TokenSemicolon)
		case ':':
			l.emitType(TokenColon)
		case '.':
			l.emitType(TokenDot)
		case '=':
			if l.acceptOne('=') {
				l.emitType(TokenEqualEqual)
			} else {
				l.emitType(TokenEqual)
			}
		case '@':
			l.emitType(TokenAt)
		case '&':
			if l.acceptOne('&') {
				l.emitType(TokenAmpersandAmpersand)
			} else {
				l.emitType(TokenAmpersand)
			}
		case '^':
			l.emitType(TokenCaret)
		case '|':
			if l.acceptOne('|') {
				l.emitType(TokenVerticalBarVerticalBar)
			} else {
				l.emitType(TokenVerticalBar)
			}
		case '>':
			r = l.next()
			switch r {
			case '>':
				l.emitType(TokenGreaterGreater)
			case '=':
				l.emitType(TokenGreaterEqual)
			default:
				l.backupOne()
				l.emitType(TokenGreater)
			}
		case '_':
			return identifierState
		case ' ', '\t':
			return spaceState(false)
		case '\n':
			return spaceState(true)
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return numberState
		case '"':
			return stringState
		case '/':
			r = l.next()
			switch r {
			case '/':
				return lineCommentState
			case '*':
				l.emitType(TokenBlockCommentStart)
				return blockCommentState(0)
			default:
				l.backupOne()
				l.emitType(TokenSlash)
			}
		case '?':
			r = l.next()
			switch r {
			case '?':
				l.emitType(TokenDoubleQuestionMark)
			case '.':
				l.emitType(TokenQuestionMarkDot)
			default:
				l.backupOne()
				l.emitType(TokenQuestionMark)
			}
		case '!':
			if l.acceptOne('=') {
				l.emitType(TokenNotEqual)
			} else {
				l.emitType(TokenExclamationMark)
			}
		case '<':
			r = l.next()
			switch r {
			case '-':
				r = l.next()
				switch r {
				case '!':
					l.emitType(TokenLeftArrowExclamation)
				case '>':
					l.emitType(TokenSwap)
				default:
					l.backupOne()
					l.emitType(TokenLeftArrow)
				}
			case '<':
				l.emitType(TokenLessLess)
			case '=':
				l.emitType(TokenLessEqual)
			default:
				l.backupOne()
				l.emitType(TokenLess)
			}
		default:
			switch {
			case r >= 'a' && r <= 'z' ||
				r >= 'A' && r <= 'Z':

				return identifierState

			default:
				return l.error(fmt.Errorf("unrecognized character: %#U", r))
			}
		}
	}
}

func (l *lexer) error(err error) stateFn {
	l.emitError(err)
	return nil
}

func numberState(l *lexer) stateFn {
	// lookahead is already lexed.
	// parse more, if any
	r := l.current
	if r == '0' {
		r = l.next()
		switch r {
		case 'b':
			l.scanBinaryRemainder()
			if l.endOffset-l.startOffset <= 2 {
				l.emitError(fmt.Errorf("missing digits"))
			}
			l.emitValue(TokenBinaryLiteral)

		case 'o':
			l.scanOctalRemainder()
			if l.endOffset-l.startOffset <= 2 {
				l.emitError(fmt.Errorf("missing digits"))
			}
			l.emitValue(TokenOctalLiteral)

		case 'x':
			l.scanHexadecimalRemainder()
			if l.endOffset-l.startOffset <= 2 {
				l.emitError(fmt.Errorf("missing digits"))
			}
			l.emitValue(TokenHexadecimalLiteral)

		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			tokenType := l.scanDecimalOrFixedPointRemainder()
			l.emitValue(tokenType)

		case '.':
			l.scanFixedPointRemainder()
			l.emitValue(TokenFixedPointLiteral)

		case EOF:
			l.backupOne()
			l.emitValue(TokenDecimalLiteral)

		default:
			prefixChar := r

			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				l.emitError(fmt.Errorf("invalid number literal prefix: %q", prefixChar))
				l.next()

				tokenType := l.scanDecimalOrFixedPointRemainder()
				l.emitValue(tokenType)
			} else {
				l.backupOne()
				l.emitValue(TokenDecimalLiteral)
			}
		}

	} else {
		tokenType := l.scanDecimalOrFixedPointRemainder()
		l.emitValue(tokenType)
	}

	return rootState
}

type Space struct {
	String          string
	ContainsNewline bool
}

func spaceState(startIsNewline bool) stateFn {
	return func(l *lexer) stateFn {
		containsNewline := l.scanSpace()
		containsNewline = containsNewline || startIsNewline
		l.emit(
			TokenSpace,
			Space{
				String:          l.word(),
				ContainsNewline: containsNewline,
			},
			l.startPosition(),
			true,
		)
		return rootState
	}
}

func identifierState(l *lexer) stateFn {
	l.scanIdentifier()
	if l.word() == keywordAs {
		r := l.next()
		switch r {
		case '?':
			l.emitType(TokenAsQuestionMark)
			return rootState
		case '!':
			l.emitType(TokenAsExclamationMark)
			return rootState
		default:
			l.backupOne()
		}
	}
	l.emitValue(TokenIdentifier)
	return rootState
}

func stringState(l *lexer) stateFn {
	l.scanString('"')
	l.emitValue(TokenString)
	return rootState
}

func lineCommentState(l *lexer) stateFn {
	l.scanLineComment()
	l.emitValue(TokenLineComment)
	return rootState
}

func blockCommentState(nesting int) stateFn {
	if nesting < 0 {
		return rootState
	}

	return func(l *lexer) stateFn {
		r := l.next()
		switch r {
		case EOF:
			l.emitType(TokenEOF)
			return nil
		case '/':
			beforeSlashOffset := l.prevEndOffset
			if l.acceptOne('*') {
				starOffset := l.endOffset
				l.endOffset = beforeSlashOffset
				l.emitValue(TokenBlockCommentContent)
				l.endOffset = starOffset
				l.emitType(TokenBlockCommentStart)
				return blockCommentState(nesting + 1)
			}

		case '*':
			beforeStarOffset := l.prevEndOffset
			if l.acceptOne('/') {
				slashOffset := l.endOffset
				l.endOffset = beforeStarOffset
				l.emitValue(TokenBlockCommentContent)
				l.endOffset = slashOffset
				l.emitType(TokenBlockCommentEnd)
				return blockCommentState(nesting - 1)
			}
		}

		return blockCommentState(nesting)
	}
}
