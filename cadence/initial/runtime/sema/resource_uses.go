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
	"github.com/raviqqe/hamt"

	"github.com/onflow/cadence-initial/runtime/ast"
)

type ResourceUse struct {
	UseAfterInvalidationReported bool
}

type ResourceUseEntry struct {
	ast.Position
}

func (e ResourceUseEntry) Equal(other hamt.Entry) bool {
	return e.Position == other.(ResourceUseEntry).Position
}

////

type ResourceUses struct {
	positions hamt.Map
}

func (p ResourceUses) AllPositions() (result []ast.Position) {
	s := p.positions
	for s.Size() != 0 {
		var e hamt.Entry
		e, _, s = s.FirstRest()
		position := e.(ResourceUseEntry).Position
		result = append(result, position)
	}
	return
}

func (p ResourceUses) Include(pos ast.Position) bool {
	return p.positions.Include(ResourceUseEntry{pos})
}

func (p ResourceUses) Insert(pos ast.Position) ResourceUses {
	if p.Include(pos) {
		return p
	}
	entry := ResourceUseEntry{pos}
	newPositions := p.positions.Insert(entry, ResourceUse{})
	return ResourceUses{newPositions}
}

func (p ResourceUses) MarkUseAfterInvalidationReported(pos ast.Position) ResourceUses {
	entry := ResourceUseEntry{pos}
	value := p.positions.Find(entry)
	use := value.(ResourceUse)
	use.UseAfterInvalidationReported = true
	newPositions := p.positions.Insert(entry, use)
	return ResourceUses{newPositions}
}

func (p ResourceUses) IsUseAfterInvalidationReported(pos ast.Position) bool {
	entry := ResourceUseEntry{pos}
	value := p.positions.Find(entry)
	use := value.(ResourceUse)
	return use.UseAfterInvalidationReported
}

func (p ResourceUses) Merge(other ResourceUses) ResourceUses {
	newPositions := p.positions.Merge(other.positions)
	return ResourceUses{newPositions}
}

func (p ResourceUses) Size() int {
	return p.positions.Size()
}
