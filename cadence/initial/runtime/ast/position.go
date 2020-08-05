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
	"github.com/segmentio/fasthash/fnv1"
)

// Position defines a row/column within a Cadence script.
type Position struct {
	// offset, starting at 0
	Offset int
	// line number, starting at 1
	Line int
	// column number, starting at 0 (byte count)
	Column int
}

func (position Position) Shifted(length int) Position {
	return Position{
		Line:   position.Line,
		Column: position.Column + length,
		Offset: position.Offset + length,
	}
}

func (position Position) Hash() (result uint32) {
	result = fnv1.Init32
	result = fnv1.AddUint32(result, uint32(position.Offset))
	result = fnv1.AddUint32(result, uint32(position.Line))
	result = fnv1.AddUint32(result, uint32(position.Column))
	return
}

func (position Position) Compare(other Position) int {
	switch {
	case position.Offset < other.Offset:
		return -1
	case position.Offset > other.Offset:
		return 1
	default:
		return 0
	}
}

func EndPosition(startPosition Position, end int) Position {
	length := end - startPosition.Offset
	return startPosition.Shifted(length)
}

// HasPosition

type HasPosition interface {
	StartPosition() Position
	EndPosition() Position
}

// Range

type Range struct {
	StartPos Position
	EndPos   Position
}

func (e *Range) StartPosition() Position {
	return e.StartPos
}

func (e *Range) EndPosition() Position {
	return e.EndPos
}

// NewRangeFromPositioned

func NewRangeFromPositioned(hasPosition HasPosition) Range {
	return Range{
		StartPos: hasPosition.StartPosition(),
		EndPos:   hasPosition.EndPosition(),
	}
}
