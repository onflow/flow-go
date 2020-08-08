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

package sema

import (
	"fmt"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/common/intervalst"
)

type Position struct {
	// line number, starting at 1
	Line int
	// column number, starting at 0 (byte count)
	Column int
}

func (pos Position) String() string {
	return fmt.Sprintf("Position{%d, %d}", pos.Line, pos.Column)
}

func (pos Position) Compare(other intervalst.Position) int {
	if _, ok := other.(intervalst.MinPosition); ok {
		return 1
	}

	otherL, ok := other.(Position)
	if !ok {
		panic(fmt.Sprintf("not a sema.Position: %#+v", other))
	}
	if pos.Line < otherL.Line {
		return -1
	}
	if pos.Line > otherL.Line {
		return 1
	}
	if pos.Column < otherL.Column {
		return -1
	}
	if pos.Column > otherL.Column {
		return 1
	}
	return 0
}

type Origin struct {
	Type            Type
	DeclarationKind common.DeclarationKind
	StartPos        *ast.Position
	EndPos          *ast.Position
}

type Occurrences struct {
	T *intervalst.IntervalST
}

func NewOccurrences() *Occurrences {
	return &Occurrences{
		T: &intervalst.IntervalST{},
	}
}

func ToPosition(position ast.Position) Position {
	return Position{
		Line:   position.Line,
		Column: position.Column,
	}
}

func (o *Occurrences) Put(startPos, endPos ast.Position, origin *Origin) {
	occurrence := Occurrence{
		StartPos: ToPosition(startPos),
		EndPos:   ToPosition(endPos),
		Origin:   origin,
	}
	interval := intervalst.NewInterval(
		occurrence.StartPos,
		occurrence.EndPos,
	)
	o.T.Put(interval, occurrence)
}

type Occurrence struct {
	StartPos Position
	EndPos   Position
	Origin   *Origin
}

func (o *Occurrences) All() []Occurrence {
	values := o.T.Values()
	occurrences := make([]Occurrence, len(values))
	for i, value := range values {
		occurrences[i] = value.(Occurrence)
	}
	return occurrences
}

func (o *Occurrences) Find(pos Position) *Occurrence {
	interval, value := o.T.Search(pos)
	if interval == nil {
		return nil
	}
	occurrence := value.(Occurrence)
	return &occurrence
}
