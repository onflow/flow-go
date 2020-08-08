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
	"github.com/segmentio/fasthash/fnv1"

	"example.com/cadence-initial/runtime/ast"
)

type ResourceInvalidation struct {
	Kind     ResourceInvalidationKind
	StartPos ast.Position
	EndPos   ast.Position
}

// ResourceInvalidationEntry allows using resource invalidations as entries in `hamt` structures
//
type ResourceInvalidationEntry struct {
	ResourceInvalidation
}

func (e ResourceInvalidationEntry) Hash() (result uint32) {
	result = fnv1.Init32
	result = fnv1.AddUint32(result, uint32(e.ResourceInvalidation.Kind))
	result = fnv1.AddUint32(result, e.ResourceInvalidation.StartPos.Hash())
	result = fnv1.AddUint32(result, e.ResourceInvalidation.EndPos.Hash())
	return
}

func (e ResourceInvalidationEntry) Equal(e2 hamt.Entry) bool {
	other := e2.(ResourceInvalidationEntry)
	return e == other
}
