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

package runtime

import (
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/interpreter"
	"github.com/onflow/cadence-initial/runtime/sema"
)

// An exportableValue is a Cadence value emitted by the runtime.
//
// Runtime values can be exported to a simplified representation then further
// encoded for transport or use in other environments.
type exportableValue struct {
	interpreter.Value
	inter *interpreter.Interpreter
}

func newExportableValue(v interpreter.Value, inter *interpreter.Interpreter) exportableValue {
	return exportableValue{
		Value: v,
		inter: inter,
	}
}

func (v exportableValue) Interpreter() *interpreter.Interpreter {
	return v.inter
}

type exportableEvent struct {
	Type   sema.Type
	Fields []exportableValue
}

type Address = common.Address
