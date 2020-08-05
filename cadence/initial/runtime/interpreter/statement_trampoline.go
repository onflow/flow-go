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

package interpreter

import (
	"github.com/onflow/cadence-initial/runtime/trampoline"
)

type StatementTrampoline struct {
	F           func() trampoline.Trampoline
	Interpreter *Interpreter
	Line        int
}

func (m StatementTrampoline) Resume() interface{} {
	return m.F
}

func (m StatementTrampoline) FlatMap(f func(interface{}) trampoline.Trampoline) trampoline.Trampoline {
	return trampoline.FlatMap{Subroutine: m, Continuation: f}
}

func (m StatementTrampoline) Map(f func(interface{}) interface{}) trampoline.Trampoline {
	return trampoline.MapTrampoline(m, f)
}

func (m StatementTrampoline) Then(f func(interface{})) trampoline.Trampoline {
	return trampoline.ThenTrampoline(m, f)
}

func (m StatementTrampoline) Continue() trampoline.Trampoline {
	return m.F()
}
