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

package common

import (
	"github.com/raviqqe/hamt"
	"github.com/segmentio/fasthash/fnv1a"
)

type StringEntry string

func (key StringEntry) Hash() uint32 {
	return fnv1a.HashString32(string(key))
}

func (key StringEntry) Equal(other hamt.Entry) bool {
	otherKey, isPointerKey := other.(StringEntry)
	return isPointerKey && string(otherKey) == string(key)
}
