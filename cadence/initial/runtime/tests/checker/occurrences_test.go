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

package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/utils"
)

// TODO: implement occurrences for type references
//  (e.g. conformances, conditional casting expression)

func TestCheckOccurrencesVariableDeclarations(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
        let x = 1
        var y = x
    `)

	require.NoError(t, err)

	occurrences := checker.Occurrences.All()

	matchers := []*OccurrenceMatcher{
		{
			StartPos:        sema.Position{Line: 2, Column: 12},
			EndPos:          sema.Position{Line: 2, Column: 12},
			OriginStartPos:  &sema.Position{Line: 2, Column: 12},
			OriginEndPos:    &sema.Position{Line: 2, Column: 12},
			DeclarationKind: common.DeclarationKindConstant,
		},
		{
			StartPos:        sema.Position{Line: 3, Column: 12},
			EndPos:          sema.Position{Line: 3, Column: 12},
			OriginStartPos:  &sema.Position{Line: 3, Column: 12},
			OriginEndPos:    &sema.Position{Line: 3, Column: 12},
			DeclarationKind: common.DeclarationKindVariable,
		},
		{
			StartPos:        sema.Position{Line: 3, Column: 16},
			EndPos:          sema.Position{Line: 3, Column: 16},
			OriginStartPos:  &sema.Position{Line: 2, Column: 12},
			OriginEndPos:    &sema.Position{Line: 2, Column: 12},
			DeclarationKind: common.DeclarationKindConstant,
		},
	}

nextMatcher:
	for _, matcher := range matchers {
		for _, occurrence := range occurrences {
			if matcher.Match(occurrence) {
				continue nextMatcher
			}
		}

		assert.Fail(t, "failed to find occurrence", "matcher: %#+v", matcher)
	}

	for _, matcher := range matchers {
		assert.NotNil(t, checker.Occurrences.Find(matcher.StartPos))
		assert.NotNil(t, checker.Occurrences.Find(matcher.EndPos))
	}
}

func TestCheckOccurrencesFunction(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
		fun f1(paramX: Int, paramY: Bool) {
		   let x = 1
		   var y: Int? = x
		   fun f2() {
		       if let y = y {
		       }
		   }
           f1(paramX: 1, paramY: true)
		}

        fun f3() {
            f1(paramX: 2, paramY: false)
        }
	`)

	require.NoError(t, err)

	occurrences := checker.Occurrences.All()

	matchers := []*OccurrenceMatcher{
		{
			StartPos:        sema.Position{Line: 2, Column: 6},
			EndPos:          sema.Position{Line: 2, Column: 7},
			OriginStartPos:  &sema.Position{Line: 2, Column: 6},
			OriginEndPos:    &sema.Position{Line: 2, Column: 6},
			DeclarationKind: common.DeclarationKindFunction,
		},
		{
			StartPos:        sema.Position{Line: 2, Column: 9},
			EndPos:          sema.Position{Line: 2, Column: 14},
			OriginStartPos:  &sema.Position{Line: 2, Column: 9},
			OriginEndPos:    &sema.Position{Line: 2, Column: 9},
			DeclarationKind: common.DeclarationKindParameter,
		},
		{
			StartPos:        sema.Position{Line: 2, Column: 22},
			EndPos:          sema.Position{Line: 2, Column: 27},
			OriginStartPos:  &sema.Position{Line: 2, Column: 22},
			OriginEndPos:    &sema.Position{Line: 2, Column: 22},
			DeclarationKind: common.DeclarationKindParameter,
		},
		{
			StartPos:        sema.Position{Line: 3, Column: 9},
			EndPos:          sema.Position{Line: 3, Column: 9},
			OriginStartPos:  &sema.Position{Line: 3, Column: 9},
			OriginEndPos:    &sema.Position{Line: 3, Column: 9},
			DeclarationKind: common.DeclarationKindConstant,
		},
		{
			StartPos:        sema.Position{Line: 4, Column: 9},
			EndPos:          sema.Position{Line: 4, Column: 9},
			OriginStartPos:  &sema.Position{Line: 4, Column: 9},
			OriginEndPos:    &sema.Position{Line: 4, Column: 9},
			DeclarationKind: common.DeclarationKindVariable,
		},
		{
			StartPos:        sema.Position{Line: 4, Column: 19},
			EndPos:          sema.Position{Line: 4, Column: 19},
			OriginStartPos:  &sema.Position{Line: 3, Column: 9},
			OriginEndPos:    &sema.Position{Line: 3, Column: 9},
			DeclarationKind: common.DeclarationKindConstant,
		},
		{
			StartPos:        sema.Position{Line: 5, Column: 9},
			EndPos:          sema.Position{Line: 5, Column: 10},
			OriginStartPos:  &sema.Position{Line: 5, Column: 9},
			OriginEndPos:    &sema.Position{Line: 5, Column: 9},
			DeclarationKind: common.DeclarationKindFunction,
		},
		{
			StartPos:        sema.Position{Line: 6, Column: 16},
			EndPos:          sema.Position{Line: 6, Column: 16},
			OriginStartPos:  &sema.Position{Line: 6, Column: 16},
			OriginEndPos:    &sema.Position{Line: 6, Column: 16},
			DeclarationKind: common.DeclarationKindConstant,
		},
		{
			StartPos:        sema.Position{Line: 6, Column: 20},
			EndPos:          sema.Position{Line: 6, Column: 20},
			OriginStartPos:  &sema.Position{Line: 4, Column: 9},
			OriginEndPos:    &sema.Position{Line: 4, Column: 9},
			DeclarationKind: common.DeclarationKindVariable,
		},
		{
			StartPos:        sema.Position{Line: 9, Column: 11},
			EndPos:          sema.Position{Line: 9, Column: 12},
			OriginStartPos:  &sema.Position{Line: 2, Column: 6},
			OriginEndPos:    &sema.Position{Line: 2, Column: 6},
			DeclarationKind: common.DeclarationKindFunction,
		},
		{
			StartPos:        sema.Position{Line: 12, Column: 12},
			EndPos:          sema.Position{Line: 12, Column: 13},
			OriginStartPos:  &sema.Position{Line: 12, Column: 12},
			OriginEndPos:    &sema.Position{Line: 12, Column: 12},
			DeclarationKind: common.DeclarationKindFunction,
		},
		{
			StartPos:        sema.Position{Line: 13, Column: 12},
			EndPos:          sema.Position{Line: 13, Column: 13},
			OriginStartPos:  &sema.Position{Line: 2, Column: 6},
			OriginEndPos:    &sema.Position{Line: 2, Column: 6},
			DeclarationKind: common.DeclarationKindFunction,
		},
	}

	ms := make([]interface{}, len(matchers))
	for i := range matchers {
		ms[i] = matchers[i]
	}

nextMatcher:
	for _, matcher := range matchers {
		for _, occurrence := range occurrences {
			if matcher.Match(occurrence) {
				continue nextMatcher
			}
		}

		assert.Fail(t, "failed to find occurrence", "matcher: %#+v", matcher)
	}

	for _, matcher := range matchers {
		assert.NotNil(t, checker.Occurrences.Find(matcher.StartPos))
		assert.NotNil(t, checker.Occurrences.Find(matcher.EndPos))
	}
}

func TestCheckOccurrencesStructAndInterface(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
		struct interface I1 {}

	    struct S1: I1 {
	       let x: Int
	       init() {
	          self.x = 1
	          self.test()
	       }
	       fun test() {}
	    }

	    fun f(): S1 {
	       return S1()
	    }
	`)

	require.NoError(t, err)

	occurrences := checker.Occurrences.All()

	matchers := []*OccurrenceMatcher{
		{
			StartPos:        sema.Position{Line: 2, Column: 19},
			EndPos:          sema.Position{Line: 2, Column: 20},
			OriginStartPos:  &sema.Position{Line: 2, Column: 19},
			OriginEndPos:    &sema.Position{Line: 2, Column: 19},
			DeclarationKind: common.DeclarationKindStructureInterface,
		},
		{
			StartPos:        sema.Position{Line: 4, Column: 12},
			EndPos:          sema.Position{Line: 4, Column: 13},
			OriginStartPos:  &sema.Position{Line: 4, Column: 12},
			OriginEndPos:    &sema.Position{Line: 4, Column: 12},
			DeclarationKind: common.DeclarationKindStructure,
		},
		{
			StartPos:        sema.Position{Line: 5, Column: 8},
			EndPos:          sema.Position{Line: 5, Column: 17},
			OriginStartPos:  &sema.Position{Line: 5, Column: 12},
			OriginEndPos:    &sema.Position{Line: 5, Column: 12},
			DeclarationKind: common.DeclarationKindField,
		},
		{
			StartPos:        sema.Position{Line: 7, Column: 11},
			EndPos:          sema.Position{Line: 7, Column: 14},
			OriginStartPos:  nil,
			OriginEndPos:    nil,
			DeclarationKind: common.DeclarationKindSelf,
		},
		{
			StartPos:        sema.Position{Line: 7, Column: 16},
			EndPos:          sema.Position{Line: 7, Column: 16},
			OriginStartPos:  &sema.Position{Line: 5, Column: 12},
			OriginEndPos:    &sema.Position{Line: 5, Column: 12},
			DeclarationKind: common.DeclarationKindField,
		},
		{
			StartPos:        sema.Position{Line: 8, Column: 11},
			EndPos:          sema.Position{Line: 8, Column: 14},
			OriginStartPos:  nil,
			OriginEndPos:    nil,
			DeclarationKind: common.DeclarationKindSelf,
		},
		{
			StartPos:        sema.Position{Line: 8, Column: 16},
			EndPos:          sema.Position{Line: 8, Column: 19},
			OriginStartPos:  &sema.Position{Line: 10, Column: 12},
			OriginEndPos:    &sema.Position{Line: 10, Column: 15},
			DeclarationKind: common.DeclarationKindFunction,
		},
		// TODO: why the duplicate?
		{
			StartPos:        sema.Position{Line: 10, Column: 12},
			EndPos:          sema.Position{Line: 10, Column: 15},
			OriginStartPos:  &sema.Position{Line: 10, Column: 12},
			OriginEndPos:    &sema.Position{Line: 10, Column: 15},
			DeclarationKind: common.DeclarationKindFunction,
		},
		{
			StartPos:        sema.Position{Line: 13, Column: 9},
			EndPos:          sema.Position{Line: 13, Column: 9},
			OriginStartPos:  &sema.Position{Line: 13, Column: 9},
			OriginEndPos:    &sema.Position{Line: 13, Column: 9},
			DeclarationKind: common.DeclarationKindFunction,
		},
		{
			StartPos:        sema.Position{Line: 14, Column: 15},
			EndPos:          sema.Position{Line: 14, Column: 16},
			OriginStartPos:  &sema.Position{Line: 4, Column: 12},
			OriginEndPos:    &sema.Position{Line: 4, Column: 12},
			DeclarationKind: common.DeclarationKindStructure,
		},
	}

	ms := make([]interface{}, len(matchers))
	for i := range matchers {
		ms[i] = matchers[i]
	}

nextMatcher:
	for _, matcher := range matchers {
		for _, occurrence := range occurrences {
			if matcher.Match(occurrence) {
				continue nextMatcher
			}
		}

		assert.Fail(t, "failed to find occurrence", "matcher: %#+v", matcher)
	}

	for _, matcher := range matchers {
		assert.NotNil(t, checker.Occurrences.Find(matcher.StartPos))
		assert.NotNil(t, checker.Occurrences.Find(matcher.EndPos))
	}
}
